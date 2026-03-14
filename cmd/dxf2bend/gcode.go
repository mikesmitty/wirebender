package main

import (
	"fmt"
	"hash/fnv"
	"math"
	"os"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/yofu/dxf/drawing"
	"github.com/yofu/dxf/entity"
)

// extractPaths pulls coordinates from all supported entity types and returns all paths found.
func extractPaths(d *drawing.Drawing, arcResolution float64, verbose bool) [][]mgl64.Vec3 {
	var segments [][]mgl64.Vec3

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
			pts := discretizeArc(ent.Center, ent.Radius, ent.Angle[0], ent.Angle[1], arcResolution, ent.Direction)
			if len(pts) > 1 {
				segments = append(segments, pts)
			}
		case *entity.Circle:
			if verbose {
				fmt.Fprintf(os.Stderr, "Found Circle: center=(%.2f,%.2f,%.2f) r=%.2f\n",
					ent.Center[0], ent.Center[1], ent.Center[2], ent.Radius)
			}
			// Treat as full 360° arc
			pts := discretizeArc(ent.Center, ent.Radius, 0, 360, arcResolution, ent.Direction)
			if len(pts) > 1 {
				segments = append(segments, pts)
			}
		case *entity.Spline:
			if verbose {
				fmt.Fprintf(os.Stderr, "Found Spline: degree=%d, %d control points, %d knots, %d fit points\n",
					ent.Degree, len(ent.Controls), len(ent.Knots), len(ent.Fits))
			}
			pts := evaluateSpline(ent)
			if len(pts) > 1 {
				segments = append(segments, pts)
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
				segments = append(segments, pts)
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
				segments = append(segments, pts)
			}
		case *entity.Line:
			pts := []mgl64.Vec3{
				{ent.Start[0], ent.Start[1], ent.Start[2]},
				{ent.End[0], ent.End[1], ent.End[2]},
			}
			segments = append(segments, pts)
		}
	}

	var paths [][]mgl64.Vec3
	if len(segments) > 0 {
		if verbose {
			fmt.Fprintf(os.Stderr, "Found %d segments, chaining...\n", len(segments))
		}
		used := make(map[int]bool)
		for i := 0; i < len(segments); i++ {
			if used[i] {
				continue
			}
			// Start a new chain
			pts := segments[i]
			used[i] = true

			// Keep growing the chain from both ends if necessary
			for {
				found := false
				head := pts[0]
				tail := pts[len(pts)-1]

				for j, seg := range segments {
					if used[j] {
						continue
					}
					segHead := seg[0]
					segTail := seg[len(seg)-1]

					// Try to append to tail
					if tail.Sub(segHead).Len() < 1e-6 {
						pts = append(pts, seg[1:]...)
						used[j] = true
						found = true
						break
					}
					if tail.Sub(segTail).Len() < 1e-6 {
						// append reversed
						for k := len(seg) - 2; k >= 0; k-- {
							pts = append(pts, seg[k])
						}
						used[j] = true
						found = true
						break
					}
					// Try to prepend to head
					if head.Sub(segTail).Len() < 1e-6 {
						pts = append(seg[:len(seg)-1], pts...)
						used[j] = true
						found = true
						break
					}
					if head.Sub(segHead).Len() < 1e-6 {
						// prepend reversed
						var rev []mgl64.Vec3
						for k := len(seg) - 1; k >= 1; k-- {
							rev = append(rev, seg[k])
						}
						pts = append(rev, pts...)
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
// Calculate OCS X and Y axes given the extrusion vector (OCS Z-axis) using the Arbitrary Axis Algorithm
func ocsAxes(extrusion []float64) (x, y mgl64.Vec3) {
	if len(extrusion) < 3 {
		return mgl64.Vec3{1, 0, 0}, mgl64.Vec3{0, 1, 0}
	}
	z := mgl64.Vec3{extrusion[0], extrusion[1], extrusion[2]}.Normalize()

	// Arbitrary Axis Algorithm threshold
	if math.Abs(z[0]) < 1.0/64.0 && math.Abs(z[1]) < 1.0/64.0 {
		wy := mgl64.Vec3{0, 1, 0}
		x = wy.Cross(z).Normalize()
	} else {
		wz := mgl64.Vec3{0, 0, 1}
		x = wz.Cross(z).Normalize()
	}

	y = z.Cross(x).Normalize()
	return x, y
}

func discretizeArc(center []float64, radius float64, startAngleDeg, endAngleDeg float64, stepDeg float64, extrusion []float64) []mgl64.Vec3 {
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

	// Get OCS axes
	ax, ay := ocsAxes(extrusion)
	az := mgl64.Vec3{0, 0, 1}
	if len(extrusion) >= 3 {
		az = mgl64.Vec3{extrusion[0], extrusion[1], extrusion[2]}.Normalize()
	}

	// Transform OCS center to WCS
	wcsCx := cx*ax[0] + cy*ay[0] + cz*az[0]
	wcsCy := cx*ax[1] + cy*ay[1] + cz*az[1]
	wcsCz := cx*ax[2] + cy*ay[2] + cz*az[2]

	pts := make([]mgl64.Vec3, nSteps+1)
	for i := 0; i <= nSteps; i++ {
		angleDeg := startAngleDeg + sweep*float64(i)/float64(nSteps)
		angleRad := angleDeg * math.Pi / 180.0

		// Point in OCS (relative to origin)
		px_ocs := radius * math.Cos(angleRad)
		py_ocs := radius * math.Sin(angleRad)

		// Transform point displacement to WCS and add to WCS center
		pts[i] = mgl64.Vec3{
			wcsCx + px_ocs*ax[0] + py_ocs*ay[0],
			wcsCy + px_ocs*ax[1] + py_ocs*ay[1],
			wcsCz + px_ocs*ax[2] + py_ocs*ay[2],
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

// WirePath represents the calculated bending operations for a wire.
type WirePath struct {
	Points       []mgl64.Vec3
	TotalRadius  float64
	Segments     []mgl64.Vec3
	Bends        []WireBend
	MaterialName string
	TotalLength  float64
	Warnings     int
}

// WireBend holds information about a single bend.
type WireBend struct {
	Index          int // Segment index where this bend occurs
	Angle          float64
	CommandedAngle float64
	Rotation       float64
	TangentDist    float64
}

// calculateWirePath computes the bend geometry for a list of points.
func calculateWirePath(points []mgl64.Vec3, springbackM, springbackO, totalRadius float64, materialName string, strict bool, verbose bool) (*WirePath, error) {
	wp := &WirePath{
		Points:       points,
		TotalRadius:  totalRadius,
		MaterialName: materialName,
	}

	segments := make([]mgl64.Vec3, len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		segments[i] = points[i+1].Sub(points[i])
	}
	wp.Segments = segments

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

	lastNormal := mgl64.Vec3{0, 0, 0}
	hasLastNormal := false
	currentRotate := 0.0

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
			wp.Warnings++
			pStart := points[i]
			pEnd := points[i+1]
			requiredTangent := 0.0
			if i > 0 {
				requiredTangent += tangentDistances[i-1]
			}
			if i < len(segments)-1 {
				requiredTangent += tangentDistances[i]
			}
			msg := fmt.Sprintf("WARNING: Segment %d is too short for bend radius.\n"+
				"  Endpoints: (%.3f, %.3f, %.3f) -> (%.3f, %.3f, %.3f)\n"+
				"  Segment length: %.3f mm, required tangent distance: %.3f mm, deficit: %.3f mm\n",
				i, pStart[0], pStart[1], pStart[2], pEnd[0], pEnd[1], pEnd[2],
				lSegment, requiredTangent, -lStraight)
			if strict {
				return nil, fmt.Errorf("%sClamping not allowed in strict mode", msg)
			}
			fmt.Fprint(os.Stderr, msg+"  Clamping straight distance to 0.\n")
			lStraight = 0
		}

		wp.TotalLength += lStraight

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

			commandedBend := bendAngle*springbackM + springbackO

			wp.Bends = append(wp.Bends, WireBend{
				Index:          i,
				Angle:          bendAngle,
				CommandedAngle: commandedBend,
				Rotation:       currentRotate,
				TangentDist:    tangentDistances[i],
			})

			// Add the arc length of the bend to the total length
			if totalRadius > 0 {
				lArc := totalRadius * (bendAngle * math.Pi / 180.0)
				wp.TotalLength += lArc
			}
		}
	}

	return wp, nil
}

// generateGCodeFromPath converts a WirePath into Wirebender G-code.
func generateGCodeFromPath(wp *WirePath, feedScale float64, speedStraight, speedBend int, springbackM, springbackO float64, verbose bool) (string, error) {
	var out string
	out += "; Generated by dxf2bend\n"
	if wp.MaterialName != "" {
		out += fmt.Sprintf("; Material: %s\n", wp.MaterialName)
	}
	out += fmt.Sprintf("; Springback: %.2f*angle + %.2f\n", springbackM, springbackO)
	out += fmt.Sprintf("; Total Radius: %.2f mm\n", wp.TotalRadius)
	if speedStraight == speedBend {
		out += fmt.Sprintf("; Speed: %d\n", speedStraight)
	} else {
		out += fmt.Sprintf("; Speed straight: %d, bend: %d\n", speedStraight, speedBend)
	}
	out += "G90 ; Absolute mode\n"
	out += "G28 ; Home all axes\n"

	currentFeed := 0.0
	numBends := 0
	minBendAngle := math.MaxFloat64
	maxBendAngle := -math.MaxFloat64

	bendMap := make(map[int]WireBend)
	for _, b := range wp.Bends {
		bendMap[b.Index] = b
	}

	for i := 0; i < len(wp.Segments); i++ {
		lSegment := wp.Segments[i].Len()
		lStraight := lSegment
		if i > 0 {
			lStraight -= bendMap[i-1].TangentDist
		}
		if i < len(wp.Segments)-1 {
			lStraight -= bendMap[i].TangentDist
		}
		if lStraight < 0 {
			lStraight = 0
		}

		currentFeed += lStraight * feedScale
		out += fmt.Sprintf("G1 L%.2f S%d\n", currentFeed, speedStraight)

		if b, ok := bendMap[i]; ok {
			numBends++
			if b.Angle < minBendAngle {
				minBendAngle = b.Angle
			}
			if b.Angle > maxBendAngle {
				maxBendAngle = b.Angle
			}

			out += fmt.Sprintf("G1 B%.2f R%.2f S%d ; Bend (desired %.2f)\n", b.CommandedAngle, b.Rotation, speedBend, b.Angle)
			out += fmt.Sprintf("G1 B0 S%d\n", speedBend)

			if wp.TotalRadius > 0 {
				lArc := wp.TotalRadius * (b.Angle * math.Pi / 180.0)
				currentFeed += lArc * feedScale
			}
		}
	}

	// Print summary to stderr
	fmt.Fprintf(os.Stderr, "\n--- Summary ---\n")
	fmt.Fprintf(os.Stderr, "Total wire length: %.2f mm\n", wp.TotalLength*feedScale)
	fmt.Fprintf(os.Stderr, "Number of bends:   %d\n", numBends)
	if numBends > 0 {
		fmt.Fprintf(os.Stderr, "Min bend angle:    %.2f deg\n", minBendAngle)
		fmt.Fprintf(os.Stderr, "Max bend angle:    %.2f deg\n", maxBendAngle)
	}
	fmt.Fprintf(os.Stderr, "Warnings:          %d\n", wp.Warnings)

	return out, nil
}

// hashPath generates a consistent 6-character hex ID for a path based on its coordinates.
func hashPath(points []mgl64.Vec3) string {
	h := fnv.New32a()
	for _, p := range points {
		fmt.Fprintf(h, "%.3f,%.3f,%.3f;", p[0], p[1], p[2])
	}
	return fmt.Sprintf("%06x", h.Sum32()&0xFFFFFF)
}

// calculatePathLength computes the total 3D length of a path by summing the distances between consecutive points.
func calculatePathLength(points []mgl64.Vec3) float64 {
	length := 0.0
	for i := 0; i < len(points)-1; i++ {
		length += points[i+1].Sub(points[i]).Len()
	}
	return length
}
