package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/yofu/dxf"
	"github.com/yofu/dxf/drawing"
	"github.com/yofu/dxf/entity"
)

func main() {
	inputPath := flag.String("in", "", "Input DXF file")
	outputPath := flag.String("out", "", "Output G-code file (default stdout)")
	feedScale := flag.Float64("scale", 1.0, "Scale factor for feed distance")
	speed := flag.Int("speed", 500, "Servo speed")
	verbose := flag.Bool("v", false, "Verbose output")

	// Material and Machine properties
	springbackM := flag.Float64("sm", 1.0, "Springback multiplier (commanded = desired * sm + so)")
	springbackO := flag.Float64("so", 0.0, "Springback offset in degrees")
	mandrelRadius := flag.Float64("mandrel", 0.0, "Mandrel radius in mm")
	wireDia := flag.Float64("wire", 0.0, "Wire diameter in mm")

	// Arc/Circle discretization
	arcResolution := flag.Float64("arc-resolution", 5.0, "Arc discretization step in degrees")

	// Multi-path selection
	pathIndex := flag.Int("path-index", -1, "Index of path to use (-1 = first path)")
	combinePaths := flag.Bool("combine-paths", false, "Concatenate all paths into one")

	// Path simplification
	simplifyTol := flag.Float64("simplify", 0.0, "Ramer-Douglas-Peucker simplification tolerance in mm (0 = disabled)")

	// Reverse path
	reversePath := flag.Bool("reverse", false, "Reverse the point order before generating G-code")

	flag.Parse()

	if *inputPath == "" {
		fmt.Println("Usage: dxf2bend -in <input.dxf> [options]")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	d, err := dxf.FromFile(*inputPath)
	if err != nil {
		log.Fatalf("Failed to load DXF: %v", err)
	}

	paths := extractPaths(d, *arcResolution, *verbose)

	if len(paths) == 0 {
		log.Fatal("No paths found in DXF")
	}

	// Report all paths
	if len(paths) > 1 {
		fmt.Fprintf(os.Stderr, "WARNING: Found %d separate paths in DXF:\n", len(paths))
		for i, p := range paths {
			fmt.Fprintf(os.Stderr, "  Path %d: %d points\n", i, len(p))
		}
	}

	// Select or combine paths
	var points []mgl64.Vec3
	if *combinePaths {
		if *verbose {
			fmt.Fprintf(os.Stderr, "Combining all %d paths\n", len(paths))
		}
		for _, p := range paths {
			points = append(points, p...)
		}
	} else {
		idx := 0
		if *pathIndex >= 0 {
			idx = *pathIndex
		}
		if idx >= len(paths) {
			log.Fatalf("Path index %d out of range (have %d paths)", idx, len(paths))
		}
		if len(paths) > 1 && *pathIndex < 0 {
			fmt.Fprintf(os.Stderr, "Using first path (use -path-index or -combine-paths to select)\n")
		}
		points = paths[idx]
	}

	if len(points) < 2 {
		log.Fatal("Selected path too short (need at least 2 points)")
	}

	// Path simplification
	if *simplifyTol > 0 {
		before := len(points)
		points = rdpSimplify(points, *simplifyTol)
		if *verbose {
			fmt.Fprintf(os.Stderr, "Simplified path: %d -> %d points (tolerance %.3f mm)\n", before, len(points), *simplifyTol)
		}
	}

	// Reverse path
	if *reversePath {
		if *verbose {
			fmt.Fprintf(os.Stderr, "Reversing path (%d points)\n", len(points))
		}
		for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
	}

	totalRadius := *mandrelRadius + (*wireDia / 2.0)
	gcode := generateGCode(points, *feedScale, *speed, *springbackM, *springbackO, totalRadius, *verbose)

	if *outputPath != "" {
		err := os.WriteFile(*outputPath, []byte(gcode), 0644)
		if err != nil {
			log.Fatalf("Failed to write output: %v", err)
		}
	} else {
		fmt.Print(gcode)
	}
}

// extractPaths pulls coordinates from all supported entity types and returns all paths found.
func extractPaths(d *drawing.Drawing, arcResolution float64, verbose bool) [][]mgl64.Vec3 {
	var paths [][]mgl64.Vec3
	var lines []struct{ start, end mgl64.Vec3 }

	if arcResolution <= 0 {
		arcResolution = 5.0
	}

	for _, e := range d.Entities() {
		switch ent := e.(type) {
		case *entity.Arc:
			if verbose {
				fmt.Fprintf(os.Stderr, "Found Arc: center=(%.2f,%.2f,%.2f) r=%.2f angles=[%.1f, %.1f]\n",
					ent.Center[0], ent.Center[1], ent.Center[2], ent.Radius, ent.Angle[0], ent.Angle[1])
			}
			pts := discretizeArc(ent.Center, ent.Radius, ent.Angle[0], ent.Angle[1], arcResolution)
			if len(pts) > 1 {
				paths = append(paths, pts)
			}
		case *entity.Circle:
			if verbose {
				fmt.Fprintf(os.Stderr, "Found Circle: center=(%.2f,%.2f,%.2f) r=%.2f\n",
					ent.Center[0], ent.Center[1], ent.Center[2], ent.Radius)
			}
			// Treat as full 360° arc
			pts := discretizeArc(ent.Center, ent.Radius, 0, 360, arcResolution)
			if len(pts) > 1 {
				paths = append(paths, pts)
			}
		case *entity.Spline:
			if verbose {
				fmt.Fprintf(os.Stderr, "Found Spline: degree=%d, %d control points, %d knots, %d fit points\n",
					ent.Degree, len(ent.Controls), len(ent.Knots), len(ent.Fits))
			}
			pts := evaluateSpline(ent)
			if len(pts) > 1 {
				paths = append(paths, pts)
			}
		case *entity.Polyline:
			if verbose {
				fmt.Fprintf(os.Stderr, "Found Polyline with %d vertices\n", len(ent.Vertices))
			}
			var pts []mgl64.Vec3
			for _, v := range ent.Vertices {
				var p mgl64.Vec3
				if len(v.Coord) >= 1 {
					p[0] = v.Coord[0]
				}
				if len(v.Coord) >= 2 {
					p[1] = v.Coord[1]
				}
				if len(v.Coord) >= 3 {
					p[2] = v.Coord[2]
				}
				pts = append(pts, p)
			}
			if len(pts) > 1 {
				paths = append(paths, pts)
			}
		case *entity.LwPolyline:
			if verbose {
				fmt.Fprintf(os.Stderr, "Found LwPolyline with %d vertices\n", len(ent.Vertices))
			}
			var pts []mgl64.Vec3
			for _, v := range ent.Vertices {
				var p mgl64.Vec3
				if len(v) >= 1 {
					p[0] = v[0]
				}
				if len(v) >= 2 {
					p[1] = v[1]
				}
				pts = append(pts, p)
			}
			if len(pts) > 1 {
				paths = append(paths, pts)
			}
		case *entity.Line:
			lines = append(lines, struct{ start, end mgl64.Vec3 }{
				start: mgl64.Vec3{ent.Start[0], ent.Start[1], ent.Start[2]},
				end:   mgl64.Vec3{ent.End[0], ent.End[1], ent.End[2]},
			})
		}
	}

	if len(lines) > 0 {
		if verbose {
			fmt.Fprintf(os.Stderr, "Found %d LINE entities, chaining...\n", len(lines))
		}
		used := make(map[int]bool)
		for i := 0; i < len(lines); i++ {
			if used[i] {
				continue
			}
			// Start a new chain
			var pts []mgl64.Vec3
			pts = append(pts, lines[i].start, lines[i].end)
			used[i] = true

			// Keep growing the chain from both ends if necessary
			for {
				found := false
				head := pts[0]
				tail := pts[len(pts)-1]

				for j, l := range lines {
					if used[j] {
						continue
					}
					// Try to append to tail
					if tail.Sub(l.start).Len() < 1e-6 {
						pts = append(pts, l.end)
						used[j] = true
						found = true
						break
					}
					if tail.Sub(l.end).Len() < 1e-6 {
						pts = append(pts, l.start)
						used[j] = true
						found = true
						break
					}
					// Try to prepend to head
					if head.Sub(l.start).Len() < 1e-6 {
						pts = append([]mgl64.Vec3{l.end}, pts...)
						used[j] = true
						found = true
						break
					}
					if head.Sub(l.end).Len() < 1e-6 {
						pts = append([]mgl64.Vec3{l.start}, pts...)
						used[j] = true
						found = true
						break
					}
				}
				if !found {
					break
				}
			}
			if len(pts) > 1 {
				paths = append(paths, pts)
			}
		}
	}

	return paths
}

// discretizeArc converts an arc (center, radius, start/end angles in degrees) into line segments.
func discretizeArc(center []float64, radius float64, startAngleDeg, endAngleDeg float64, stepDeg float64) []mgl64.Vec3 {
	// Normalize angles: DXF arcs go counterclockwise from start to end.
	// If end < start, the arc wraps around 360°.
	sweep := endAngleDeg - startAngleDeg
	for sweep <= 0 {
		sweep += 360.0
	}

	nSteps := int(math.Ceil(sweep / stepDeg))
	if nSteps < 1 {
		nSteps = 1
	}

	cx, cy, cz := center[0], center[1], 0.0
	if len(center) >= 3 {
		cz = center[2]
	}

	pts := make([]mgl64.Vec3, nSteps+1)
	for i := 0; i <= nSteps; i++ {
		angleDeg := startAngleDeg + sweep*float64(i)/float64(nSteps)
		angleRad := angleDeg * math.Pi / 180.0
		pts[i] = mgl64.Vec3{
			cx + radius*math.Cos(angleRad),
			cy + radius*math.Sin(angleRad),
			cz,
		}
	}
	return pts
}

// evaluateSpline approximates a B-spline by evaluating it at many parameter values.
// Uses the De Boor algorithm for B-spline evaluation.
func evaluateSpline(s *entity.Spline) []mgl64.Vec3 {
	// If we have fit points but no control points, use fit points directly
	if len(s.Controls) == 0 && len(s.Fits) > 0 {
		pts := make([]mgl64.Vec3, len(s.Fits))
		for i, f := range s.Fits {
			var p mgl64.Vec3
			if len(f) >= 1 {
				p[0] = f[0]
			}
			if len(f) >= 2 {
				p[1] = f[1]
			}
			if len(f) >= 3 {
				p[2] = f[2]
			}
			pts[i] = p
		}
		return pts
	}

	if len(s.Controls) == 0 || len(s.Knots) == 0 {
		return nil
	}

	degree := s.Degree
	knots := s.Knots
	controls := s.Controls
	n := len(controls) - 1

	// Validate knot vector length: should be n + degree + 2
	expectedKnots := n + degree + 2
	if len(knots) < expectedKnots {
		// Fallback: just return control points as polyline
		pts := make([]mgl64.Vec3, len(controls))
		for i, c := range controls {
			var p mgl64.Vec3
			if len(c) >= 1 {
				p[0] = c[0]
			}
			if len(c) >= 2 {
				p[1] = c[1]
			}
			if len(c) >= 3 {
				p[2] = c[2]
			}
			pts[i] = p
		}
		return pts
	}

	// Parameter range
	tMin := knots[degree]
	tMax := knots[n+1]

	// Number of output segments: roughly 10 per control point
	nSamples := len(controls) * 10
	if nSamples < 20 {
		nSamples = 20
	}

	pts := make([]mgl64.Vec3, 0, nSamples+1)
	for i := 0; i <= nSamples; i++ {
		t := tMin + (tMax-tMin)*float64(i)/float64(nSamples)
		p := deBoor(degree, knots, controls, t)
		pts = append(pts, p)
	}
	return pts
}

// deBoor evaluates a B-spline at parameter t using De Boor's algorithm.
func deBoor(degree int, knots []float64, controls [][]float64, t float64) mgl64.Vec3 {
	n := len(controls) - 1

	// Find knot span index k such that knots[k] <= t < knots[k+1]
	k := degree
	for k < n+1 && knots[k+1] <= t {
		k++
	}
	if k > n {
		k = n
	}

	// Copy the relevant control points
	d := make([]mgl64.Vec3, degree+1)
	for j := 0; j <= degree; j++ {
		idx := k - degree + j
		if idx < 0 {
			idx = 0
		}
		if idx > n {
			idx = n
		}
		var p mgl64.Vec3
		c := controls[idx]
		if len(c) >= 1 {
			p[0] = c[0]
		}
		if len(c) >= 2 {
			p[1] = c[1]
		}
		if len(c) >= 3 {
			p[2] = c[2]
		}
		d[j] = p
	}

	// De Boor recursion
	for r := 1; r <= degree; r++ {
		for j := degree; j >= r; j-- {
			knotIdx := k - degree + j
			denom := knots[knotIdx+degree-r+1] - knots[knotIdx]
			if math.Abs(denom) < 1e-12 {
				continue
			}
			alpha := (t - knots[knotIdx]) / denom
			for dim := 0; dim < 3; dim++ {
				d[j][dim] = (1.0-alpha)*d[j-1][dim] + alpha*d[j][dim]
			}
		}
	}

	return d[degree]
}

// rdpSimplify applies the Ramer-Douglas-Peucker algorithm to simplify a path.
func rdpSimplify(points []mgl64.Vec3, epsilon float64) []mgl64.Vec3 {
	if len(points) <= 2 {
		return points
	}

	// Find the point with the maximum distance from the line between first and last
	dmax := 0.0
	index := 0
	first := points[0]
	last := points[len(points)-1]

	for i := 1; i < len(points)-1; i++ {
		d := perpendicularDistance(points[i], first, last)
		if d > dmax {
			dmax = d
			index = i
		}
	}

	if dmax > epsilon {
		// Recursively simplify both halves
		left := rdpSimplify(points[:index+1], epsilon)
		right := rdpSimplify(points[index:], epsilon)
		// Combine (avoid duplicating the split point)
		result := make([]mgl64.Vec3, 0, len(left)+len(right)-1)
		result = append(result, left[:len(left)-1]...)
		result = append(result, right...)
		return result
	}

	// All points are within tolerance; keep only endpoints
	return []mgl64.Vec3{first, last}
}

// perpendicularDistance computes the perpendicular distance from point p to the line segment from a to b.
func perpendicularDistance(p, a, b mgl64.Vec3) float64 {
	ab := b.Sub(a)
	abLen := ab.Len()
	if abLen < 1e-12 {
		return p.Sub(a).Len()
	}
	// Cross product magnitude / line length = perpendicular distance
	ap := p.Sub(a)
	cross := mgl64.Vec3{
		ap[1]*ab[2] - ap[2]*ab[1],
		ap[2]*ab[0] - ap[0]*ab[2],
		ap[0]*ab[1] - ap[1]*ab[0],
	}
	return cross.Len() / abLen
}

// generateGCode converts 3D points into Wirebender G-code (LINEAR, BEND, ROTATE).
func generateGCode(points []mgl64.Vec3, feedScale float64, speed int, springbackM float64, springbackO float64, totalRadius float64, verbose bool) string {
	var out string
	out += "; Generated by dxf2bend\n"
	out += fmt.Sprintf("; Springback: %.2f*angle + %.2f\n", springbackM, springbackO)
	out += fmt.Sprintf("; Total Radius: %.2f mm\n", totalRadius)
	out += "G90 ; Absolute mode\n"
	out += "G28 ; Home all axes\n"

	segments := make([]mgl64.Vec3, len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		segments[i] = points[i+1].Sub(points[i])
	}

	// Pre-calculate bend angles and tangent distances
	bendAngles := make([]float64, len(segments)-1)
	tangentDistances := make([]float64, len(segments)-1)
	for i := 0; i < len(segments)-1; i++ {
		vPrev := segments[i]
		vCurr := segments[i+1]
		dot := vPrev.Normalize().Dot(vCurr.Normalize())
		if dot > 1.0 {
			dot = 1.0
		}
		if dot < -1.0 {
			dot = -1.0
		}
		angle := math.Acos(dot)
		bendAngles[i] = angle * 180.0 / math.Pi

		if totalRadius > 0 {
			tangentDistances[i] = totalRadius * math.Tan(angle/2.0)
		}
	}

	// Warn about bend angles exceeding machine limit
	for i, angle := range bendAngles {
		if angle > 180.0 {
			fmt.Fprintf(os.Stderr, "WARNING: Bend angle at segment %d is %.2f° which exceeds the 180° machine limit\n", i, angle)
		}
	}

	currentFeed := 0.0
	currentRotate := 0.0
	lastNormal := mgl64.Vec3{0, 0, 0}
	hasLastNormal := false

	for i := 0; i < len(segments); i++ {
		lSegment := segments[i].Len()

		// Adjust feed for the straight part of the segment
		lStraight := lSegment
		if i > 0 {
			// Subtract tangent distance from previous bend
			lStraight -= tangentDistances[i-1]
		}
		if i < len(segments)-1 {
			// Subtract tangent distance for next bend
			lStraight -= tangentDistances[i]
		}

		if lStraight < 0 {
			fmt.Fprintf(os.Stderr, "WARNING: Segment %d is too short (%.2f) for bend radius! Straight part: %.2f\n", i, lSegment, lStraight)
			lStraight = 0
		}

		// Feed the straight part
		currentFeed += lStraight * feedScale
		out += fmt.Sprintf("G1 L%.2f S%d\n", currentFeed, speed)

		// If this is not the last segment, we have a bend
		if i < len(segments)-1 {
			vPrev := segments[i]
			vCurr := segments[i+1]
			bendAngle := bendAngles[i]

			// Calculate rotation
			rotateDelta := 0.0
			normal := vPrev.Cross(vCurr)
			if normal.Len() > 1e-6 {
				normal = normal.Normalize()
				if hasLastNormal {
					cosPhi := lastNormal.Dot(normal)
					if cosPhi > 1.0 {
						cosPhi = 1.0
					}
					if cosPhi < -1.0 {
						cosPhi = -1.0
					}
					phi := math.Acos(cosPhi) * 180.0 / math.Pi
					signVec := lastNormal.Cross(normal)
					if signVec.Dot(vPrev.Normalize()) < 0 {
						phi = -phi
					}
					rotateDelta = phi
				}
				lastNormal = normal
				hasLastNormal = true
			}

			for rotateDelta > 180 {
				rotateDelta -= 360
			}
			for rotateDelta < -180 {
				rotateDelta += 360
			}
			currentRotate += rotateDelta

			// Apply springback to commanded angle
			commandedBend := bendAngle*springbackM + springbackO

			// G-code for rotate and bend
			out += fmt.Sprintf("G1 B%.2f R%.2f S%d ; Bend (desired %.2f)\n", commandedBend, currentRotate, speed, bendAngle)
			out += fmt.Sprintf("G1 B0 S%d\n", speed)

			// Feed the arc length of the bend
			if totalRadius > 0 {
				lArc := totalRadius * (bendAngle * math.Pi / 180.0)
				currentFeed += lArc * feedScale
			}
		}
	}

	return out
}
