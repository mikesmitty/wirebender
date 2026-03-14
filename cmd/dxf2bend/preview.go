package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image/color"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/blacktop/go-termimg"
	"github.com/fogleman/gg"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/golang/freetype/truetype"
	"github.com/spf13/cobra"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

func loadFont(dc *gg.Context, path string, size float64) error {
	fontBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Try parsing as collection (.ttc) first using opentype
	collection, err := opentype.ParseCollection(fontBytes)
	if err == nil {
		num := collection.NumFonts()
		if num > 0 {
			// Find the boldest one if we can
			bestIdx := 0
			// Basic heuristic: check font names for "Bold"
			for i := 0; i < num; i++ {
				f, err := collection.Font(i)
				if err != nil {
					continue
				}
				// opentype.Font doesn't have a name method easily, but let's try
				// to parse it as truetype just to check the name
				// This is a bit expensive but we only do it once.
				// Since we're in a collection, we can't easily parse a subfont as truetype
				// without finding its offset. Let's just assume index 0 for now
				// OR we can try indices that are commonly bold in macOS TTCs (often 1 or 2)
				if strings.Contains(strings.ToLower(path), ".ttc") {
					// In many macOS TTCs, index 0 is Regular, index 4 is Bold, or similar.
					// Let's try to check for Bold if possible.
				}
				_ = f
			}
			f, _ := collection.Font(bestIdx)
			face, err := opentype.NewFace(f, &opentype.FaceOptions{
				Size:    size,
				DPI:     72,
				Hinting: font.HintingNone,
			})
			if err == nil {
				dc.SetFontFace(face)
				return nil
			}
		}
	}

	// Fallback to single font parsing
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return err
	}

	face := truetype.NewFace(f, &truetype.Options{
		Size: size,
	})
	dc.SetFontFace(face)
	return nil
}

var previewCmd = &cobra.Command{
	Use:   "preview [input.gcode | input.dxf]",
	Short: "Generate a visual preview from a G-code or DXF file",
	Args:  cobra.ExactArgs(1),
	RunE:  runPreview,
}

func init() {
	rootCmd.AddCommand(previewCmd)
}

func runPreview(cmd *cobra.Command, args []string) error {
	if previewTarget == "none" {
		return nil
	}
	inputPath := args[0]

	if strings.HasSuffix(strings.ToLower(inputPath), ".dxf") {
		paths, ids, err := parseDXFPaths(inputPath)
		if err != nil {
			return fmt.Errorf("failed to parse DXF: %w", err)
		}

		if pathID != "" || pathIndex >= 0 || combinePaths {
			// A specific path is selected, use regular parsing and rendering
			wp, err := parseDXFToWirePath(inputPath)
			if err != nil {
				return err
			}
			return generatePreview(wp, previewTarget)
		}

		// Preview all paths
		return generatePreviewMultiple(paths, ids, previewTarget)
	} else {
		data, readErr := os.ReadFile(inputPath)
		if readErr != nil {
			return fmt.Errorf("failed to read G-code: %w", readErr)
		}

		wp, err := simulateGCode(string(data))
		if err != nil {
			return fmt.Errorf("failed to simulate G-code: %w", err)
		}
		return generatePreview(wp, previewTarget)
	}
}

type PreviewTheme struct {
	Background      color.Color
	Grid            color.Color
	GridText        color.Color
	WireStraight    color.Color
	WireArc         color.Color
	LabelBackground color.Color
	LabelBorder     color.Color
	LeaderLine      color.Color
	Colors          []color.Color // Palette for multiple paths
	BendLabel       color.Color
	ArcLabel        color.Color
	LengthLabel     color.Color
}

var (
	ThemeVibrant = PreviewTheme{
		Background:      color.White,
		Grid:            color.RGBA{220, 220, 220, 255},
		GridText:        color.RGBA{150, 150, 150, 255},
		WireStraight:    color.RGBA{20, 20, 20, 255},
		WireArc:         color.RGBA{200, 200, 200, 255},
		LabelBackground: color.NRGBA{255, 255, 255, 240},
		LabelBorder:     color.RGBA{180, 180, 180, 255},
		LeaderLine:      color.NRGBA{100, 100, 100, 180},
		Colors: []color.Color{
			color.RGBA{41, 121, 255, 255}, // Blue A400
			color.RGBA{255, 23, 68, 255},  // Red A400
			color.RGBA{0, 230, 118, 255},  // Green A400
			color.RGBA{255, 196, 0, 255},  // Amber A400
			color.RGBA{213, 0, 249, 255},  // Purple A700
			color.RGBA{0, 184, 212, 255},  // Cyan A700
			color.RGBA{255, 109, 0, 255},  // Orange A700
		},
		BendLabel:   color.RGBA{255, 23, 68, 255},  // Red A400
		ArcLabel:    color.RGBA{41, 121, 255, 255}, // Blue A400
		LengthLabel: color.RGBA{0, 150, 136, 255},  // Teal 500 (slighty darker for readability on white)
	}

	ThemeCyberpunk = PreviewTheme{
		Background:      color.RGBA{18, 18, 18, 255},
		Grid:            color.RGBA{50, 50, 50, 255},
		GridText:        color.RGBA{150, 150, 150, 255},
		WireStraight:    color.RGBA{0, 255, 255, 255}, // Cyan
		WireArc:         color.RGBA{255, 0, 255, 255}, // Hot Pink
		LabelBackground: color.NRGBA{30, 30, 30, 220},
		LabelBorder:     color.RGBA{150, 150, 150, 255},
		LeaderLine:      color.NRGBA{255, 255, 255, 100},
		Colors: []color.Color{
			color.RGBA{0, 255, 255, 255}, // Cyan
			color.RGBA{255, 0, 255, 255}, // Hot Pink
			color.RGBA{57, 255, 20, 255}, // Lime Green
			color.RGBA{255, 255, 0, 255}, // Bright Yellow
			color.RGBA{255, 0, 85, 255},  // Neon Red
			color.RGBA{157, 0, 255, 255}, // Purple
			color.RGBA{0, 255, 127, 255}, // Spring Green
		},
		BendLabel:   color.RGBA{57, 255, 20, 255}, // Lime Green
		ArcLabel:    color.RGBA{255, 0, 255, 255}, // Hot Pink
		LengthLabel: color.RGBA{255, 255, 0, 255}, // Bright Yellow
	}
)

func getTheme() PreviewTheme {
	switch strings.ToLower(previewTheme) {
	case "cyberpunk":
		return ThemeCyberpunk
	default:
		return ThemeVibrant
	}
}

type labelRect struct {
	X, Y, W, H float64
}

func (r1 labelRect) Intersects(r2 labelRect) bool {
	return !(r1.X > r2.X+r2.W || r1.X+r1.W < r2.X || r1.Y > r2.Y+r2.H || r1.Y+r1.H < r2.Y)
}

type bendGroup struct {
	bends []WireBend
}

func groupBends(bends []WireBend) []bendGroup {
	if len(bends) == 0 {
		return nil
	}

	var groups []bendGroup
	currentGroup := bendGroup{bends: []WireBend{bends[0]}}

	for i := 1; i < len(bends); i++ {
		b1 := bends[i-1]
		b2 := bends[i]

		// Bends are sequential
		isSequential := b2.Index == b1.Index+1

		// Bends have roughly the same spatial orientation/rotation
		sameRotation := math.Abs(b2.Rotation-b1.Rotation) < 0.1 || math.Abs(math.Abs(b2.Rotation-b1.Rotation)-360.0) < 0.1

		// Bends are in the same direction (both positive or both negative)
		sameDirection := (b1.Angle > 0 && b2.Angle > 0) || (b1.Angle < 0 && b2.Angle < 0)

		if isSequential && sameRotation && sameDirection {
			currentGroup.bends = append(currentGroup.bends, b2)
		} else {
			groups = append(groups, currentGroup)
			currentGroup = bendGroup{bends: []WireBend{b2}}
		}
	}
	groups = append(groups, currentGroup)

	return groups
}

// simulateGCode parses G-code and reconstructs the 3D path.
func simulateGCode(gcode string) (*WirePath, error) {
	wp := &WirePath{}

	// Regex for G1 L... B... R...
	// Note: B and R are absolute in the machine model

	// Better regex to capture all in one line
	lineRe := regexp.MustCompile(`(?i)G1\s+.*`)
	lRe := regexp.MustCompile(`(?i)L([-?\d.]+)`)
	bRe := regexp.MustCompile(`(?i)B([-?\d.]+)`)
	rRe := regexp.MustCompile(`(?i)R([-?\d.]+)`)
	commentRe := regexp.MustCompile(`(?i);\s*Bend\s*\(desired\s*([-?\d.]+)\)`)
	radRe := regexp.MustCompile(`(?i);\s*Total\s*Radius:\s*([-?\d.]+)`)

	// Initial state
	pos := mgl64.Vec3{0, 0, 0}
	forward := mgl64.Vec3{1, 0, 0}
	up := mgl64.Vec3{0, 0, 1}

	currentFeed := 0.0
	currentBend := 0.0
	currentRotate := 0.0

	scanner := bufio.NewScanner(strings.NewReader(gcode))
	for scanner.Scan() {
		line := scanner.Text()

		if m := radRe.FindStringSubmatch(line); m != nil {
			wp.TotalRadius, _ = strconv.ParseFloat(m[1], 64)
		}

		if !lineRe.MatchString(line) {
			continue
		}

		newFeed := currentFeed
		newBend := currentBend
		newRotate := currentRotate

		if m := lRe.FindStringSubmatch(line); m != nil {
			newFeed, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := bRe.FindStringSubmatch(line); m != nil {
			newBend, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := rRe.FindStringSubmatch(line); m != nil {
			newRotate, _ = strconv.ParseFloat(m[1], 64)
		}

		// Handle feed (L)
		if newFeed != currentFeed {
			dist := newFeed - currentFeed
			if dist > 0 {
				if len(wp.Points) == 0 {
					wp.Points = append(wp.Points, pos)
				}
				pos = pos.Add(forward.Mul(dist))
				wp.Points = append(wp.Points, pos)
				wp.Segments = append(wp.Segments, forward.Mul(dist))
			}
			currentFeed = newFeed
		}

		// Handle rotation (R)
		if newRotate != currentRotate {
			angle := (newRotate - currentRotate) * math.Pi / 180.0
			// Roll around forward axis
			rot := mgl64.QuatRotate(angle, forward)
			up = rot.Rotate(up).Normalize()
			currentRotate = newRotate
		}

		// Handle bend (B)
		if newBend != currentBend {
			// In our simulation, we only care about the result of the bend.
			// The machine bends, then returns to 0.
			// If newBend is 0, it means we finished a bend.
			if newBend != 0 {
				desired := newBend
				if m := commentRe.FindStringSubmatch(line); m != nil {
					desired, _ = strconv.ParseFloat(m[1], 64)
				}

				angle := desired * math.Pi / 180.0
				// Bend axis is 'up'
				rot := mgl64.QuatRotate(angle, up)
				forward = rot.Rotate(forward).Normalize()

				// Record the bend
				wp.Bends = append(wp.Bends, WireBend{
					Index:    len(wp.Segments) - 1,
					Angle:    desired,
					Rotation: currentRotate,
				})
			}
			currentBend = newBend
		}
	}

	return wp, nil
}

// generatePreview creates the visual diagram.
func generatePreview(wp *WirePath, outputPath string) error {
	const (
		width  = 1200
		height = 900
		margin = 50
	)

	// We need to build the full geometry (including arcs) for rendering
	type Node struct {
		P     mgl64.Vec3
		Type  string // "straight", "arc"
		Label string
	}

	// Actually, let's just use the vertices for now and draw them.
	// For better preview, we should discretize the arcs.

	// Project points to 2D
	type Point2D struct {
		X, Y float64
	}

	// Isometric projection
	project := func(p mgl64.Vec3) (float64, float64) {
		// Isometric view: rotate 45 deg around Y, then ~35 deg around X
		// Simplified:
		x := (p[0] - p[2]) * math.Cos(math.Pi/6)
		y := (p[0]+p[2])*math.Sin(math.Pi/6) - p[1]
		return x, y
	}

	// Calculate bounds
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64

	var pts2d []Point2D
	for _, p := range wp.Points {
		u, v := project(p)
		pts2d = append(pts2d, Point2D{u, v})
		if u < minX {
			minX = u
		}
		if u > maxX {
			maxX = u
		}
		if v < minY {
			minY = v
		}
		if v > maxY {
			maxY = v
		}
	}

	if len(pts2d) == 0 {
		return fmt.Errorf("no points to render")
	}

	// Scaling
	dx := maxX - minX
	dy := maxY - minY
	if dx == 0 {
		dx = 1
	}
	if dy == 0 {
		dy = 1
	}

	scaleX := (width - 2*margin) / dx
	scaleY := (height - 2*margin) / dy
	scale := math.Min(scaleX, scaleY)

	// Centering
	offsetX := margin + (width-2*margin-dx*scale)/2 - minX*scale
	offsetY := margin + (height-2*margin-dy*scale)/2 - minY*scale

	transform := func(p Point2D) (float64, float64) {
		return p.X*scale + offsetX, p.Y*scale + offsetY
	}

	dc := gg.NewContext(width, height)

	// Try to load a clean modern font (Bold)
	fontPaths := []string{
		"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
		"/Library/Fonts/Arial Bold.ttf",
		"/System/Library/Fonts/Helvetica-Bold.otf",
		"/System/Library/Fonts/Helvetica-Bold.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
		"C:\\Windows\\Fonts\\arialbd.ttf",
		"/System/Library/Fonts/HelveticaNeue.ttc",
		"/System/Library/Fonts/Helvetica.ttc",
		"/System/Library/Fonts/SFNS.ttf",
	}
	fontLoaded := false
	for _, p := range fontPaths {
		if err := loadFont(dc, p, 24); err == nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Loaded bold font: %s\n", p)
			}
			fontLoaded = true
			break
		}
	}
	if !fontLoaded {
		// Fallback to regular fonts if bold not found
		regFontPaths := []string{
			"/System/Library/Fonts/Helvetica.ttc",
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"C:\\Windows\\Fonts\\arial.ttf",
		}
		for _, p := range regFontPaths {
			if err := loadFont(dc, p, 24); err == nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Loaded fallback font: %s\n", p)
				}
				fontLoaded = true
				break
			}
		}
	}
	if !fontLoaded {
		fmt.Fprintf(os.Stderr, "Warning: Could not load any of the preferred fonts, falling back to default.\n")
	}

	theme := getTheme()
	dc.SetColor(theme.Background)
	dc.Clear()

	// Draw grid/axes (optional)
	dc.SetColor(theme.Grid)
	dc.SetLineWidth(1)
	// Origin axes
	oU, oV := project(mgl64.Vec3{0, 0, 0})
	oX, oY := transform(Point2D{oU, oV})

	xU, xV := project(mgl64.Vec3{50, 0, 0})
	xX, xY := transform(Point2D{xU, xV})
	dc.DrawLine(oX, oY, xX, xY)
	dc.Stroke()
	dc.SetColor(theme.GridText)
	dc.DrawStringAnchored("X", xX, xY, 0.5, 0.5)
	dc.SetColor(theme.Grid)

	yU, yV := project(mgl64.Vec3{0, 50, 0})
	yX, yY := transform(Point2D{yU, yV})
	dc.DrawLine(oX, oY, yX, yY)
	dc.Stroke()
	dc.SetColor(theme.GridText)
	dc.DrawStringAnchored("Y", yX, yY, 0.5, 0.5)
	dc.SetColor(theme.Grid)

	zU, zV := project(mgl64.Vec3{0, 0, 50})
	zX, zY := transform(Point2D{zU, zV})
	dc.DrawLine(oX, oY, zX, zY)
	dc.Stroke()
	dc.SetColor(theme.GridText)
	dc.DrawStringAnchored("Z", zX, zY, 0.5, 0.5)

	// Draw wire (distinguishing straight vs arc segments)
	dc.SetColor(theme.WireStraight)
	dc.SetLineWidth(5)
	dc.SetLineCapRound()

	// Map of segment index -> is part of an arc
	arcSegments := make(map[int]bool)
	groups := groupBends(wp.Bends)
	for _, g := range groups {
		if len(g.bends) > 1 {
			// Skip the first bend's index as it's the straight line leading into the arc
			for i := 1; i < len(g.bends); i++ {
				arcSegments[g.bends[i].Index] = true
			}
		}
	}

	for i := 0; i < len(pts2d)-1; i++ {
		x1, y1 := transform(pts2d[i])
		x2, y2 := transform(pts2d[i+1])

		if arcSegments[i] {
			dc.SetColor(theme.WireArc)
			dc.SetLineWidth(2.5) // Thinner for arc
			dc.SetDash()         // Solid line
		} else {
			dc.SetColor(theme.WireStraight)
			dc.SetLineWidth(5)
			dc.SetDash() // Solid line for straight
		}

		dc.DrawLine(x1, y1, x2, y2)
		dc.Stroke()
	}

	var drawnRects []labelRect

	// Helper to draw an annotated label with collision detection
	drawLabel := func(text string, baseX, baseY float64, preferredX, preferredY float64, c color.Color, minSegmentLen float64, currentSegmentLen float64) {
		if currentSegmentLen >= 0 && currentSegmentLen < minSegmentLen {
			return
		}

		lines := strings.Split(text, "\n")
		var sw, sh float64
		for _, line := range lines {
			w, h := dc.MeasureString(line)
			if w > sw {
				sw = w
			}
			sh += h
		}
		// Add some line spacing
		if len(lines) > 1 {
			sh += float64(len(lines)-1) * 6
		}

		pad := 6.0
		margin := 5.0

		bestX, bestY := baseX+preferredX, baseY+preferredY

		// Spiral search for non-overlapping position
		angle := 0.0
		radius := 0.0
		found := false
		for i := 0; i < 100; i++ {
			rect := labelRect{
				X: bestX - pad - margin,
				Y: bestY - sh - pad - margin,
				W: sw + (pad+margin)*2,
				H: sh + (pad+margin)*2,
			}

			overlap := false
			if rect.X < 0 || rect.Y < 0 || rect.X+rect.W > float64(width) || rect.Y+rect.H > float64(height) {
				overlap = true
			} else {
				for _, r := range drawnRects {
					if rect.Intersects(r) {
						overlap = true
						break
					}
				}
			}

			if !overlap {
				drawnRects = append(drawnRects, rect)
				found = true
				break
			}

			angle += 0.5
			radius += 5.0
			bestX = baseX + preferredX + math.Cos(angle)*radius
			bestY = baseY + preferredY + math.Sin(angle)*radius
		}

		if !found {
			return // skip if we can't find a spot
		}

		// Leader line if moved far from base point
		distFromBase := math.Sqrt(math.Pow(bestX-baseX, 2) + math.Pow(bestY-baseY, 2))
		if distFromBase > 30 {
			dc.SetColor(theme.LeaderLine)
			dc.SetDash() // Ensure leader line is solid
			dc.SetLineWidth(1)
			dc.DrawLine(baseX, baseY, bestX+sw/2, bestY-sh/2)
			dc.Stroke()
		}

		// Background
		dc.SetColor(theme.LabelBackground)
		dc.DrawRectangle(bestX-pad, bestY-sh-pad, sw+pad*2, sh+pad*2)
		dc.FillPreserve()
		dc.SetColor(theme.LabelBorder)
		dc.SetLineWidth(1)
		dc.Stroke()

		// Text
		dc.SetColor(c)
		currY := bestY - sh - 5
		for _, line := range lines {
			_, lh := dc.MeasureString(line)
			// Draw multiple times for fake bolding effect
			// Using sh/2 - (currY - (bestY - sh)) to find the relative offset
			dc.DrawStringAnchored(line, bestX+sw/2, currY+lh/2, 0.5, 0.5)
			dc.DrawStringAnchored(line, bestX+sw/2+0.5, currY+lh/2, 0.5, 0.5)
			dc.DrawStringAnchored(line, bestX+sw/2, currY+lh/2+0.5, 0.5, 0.5)
			currY += lh + 6 // line spacing
		}
	}

	// Draw annotations
	for _, g := range groups {
		if len(g.bends) == 1 {
			// Single bend
			b := g.bends[0]
			if math.Abs(b.Angle) < 0.01 {
				continue
			}
			idx := b.Index + 1
			if idx >= len(pts2d) {
				continue
			}
			x, y := transform(pts2d[idx])

			label := fmt.Sprintf("%.1f°", b.Angle)
			if b.Rotation != 0 {
				label += fmt.Sprintf(" Rot:%.1f°", b.Rotation)
			}
			drawLabel(label, x, y, 15, -35, theme.BendLabel, 0, 0)
		} else {
			// Arc group
			totalAngle := 0.0
			var totalRadiusOfArc float64
			count := 0
			for _, b := range g.bends {
				totalAngle += b.Angle
				if math.Abs(b.Angle) > 0.01 {
					l := wp.Segments[b.Index].Len()
					angleRad := b.Angle * math.Pi / 180.0
					r := l / (2.0 * math.Sin(math.Abs(angleRad)/2.0))
					totalRadiusOfArc += r
					count++
				}
			}

			var radiusLabel string
			if count > 0 {
				avgRadius := totalRadiusOfArc / float64(count)
				radiusLabel = fmt.Sprintf(" (R:%.1fmm)", avgRadius)
			}

			var midX, midY float64
			// Find physical midpoint of the arc for the label
			midIdx := g.bends[len(g.bends)/2].Index + 1
			if midIdx < len(pts2d) {
				midX, midY = transform(pts2d[midIdx])
			}

			// Use the rotation of the first bend in the sequence
			rot := g.bends[0].Rotation
			label := fmt.Sprintf("Arc: %.1f°%s\n%d segments", totalAngle, radiusLabel, len(g.bends)+1)
			if rot != 0 {
				label += fmt.Sprintf("\nRot:%.1f°", rot)
			}
			drawLabel(label, midX, midY, 25, -50, theme.ArcLabel, 0, 0)
		}
	}

	// Draw segment lengths
	for i := 0; i < len(wp.Segments); i++ {
		if i+1 >= len(pts2d) {
			continue
		}
		x1, y1 := transform(pts2d[i])
		x2, y2 := transform(pts2d[i+1])

		midX, midY := (x1+x2)/2, (y1+y2)/2
		dist := wp.Segments[i].Len()
		// Only label segments > 5mm to avoid clutter in arcs
		drawLabel(fmt.Sprintf("%.1fmm", dist), midX, midY, 0, 45, theme.LengthLabel, 5.0, dist)
	}

	// Output
	if outputPath == "terminal" {
		// Terminal preview
		var buf bytes.Buffer
		if err := dc.EncodePNG(&buf); err != nil {
			return err
		}
		return displayInTerminal(buf.Bytes())
	}

	if strings.HasSuffix(strings.ToLower(outputPath), ".svg") {
		return fmt.Errorf("SVG output not yet implemented; please use .png extension")
	}

	return dc.SavePNG(outputPath)
}

func displayInTerminal(data []byte) error {
	// Create a temporary file to use with go-termimg
	tmpFile, err := os.CreateTemp("", "wirebender-preview-*.png")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		os.Remove(tmpFile.Name())
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Use go-termimg to display the image.
	if os.Getenv("TERM_PROGRAM") == "vscode" {
		// VS Code terminal supports ITerm2 protocol but doesn't always advertise it correctly
		if img, err := termimg.Open(tmpFile.Name()); err == nil {
			if err := img.Protocol(termimg.ITerm2).Print(); err == nil {
				os.Remove(tmpFile.Name())
				return nil
			}
		}
	}

	if err := termimg.PrintFile(tmpFile.Name()); err == nil {
		// Terminal support confirmed and image printed. Safe to delete.
		os.Remove(tmpFile.Name())
		return nil
	}

	// Fallback to opening with the system default application.
	fmt.Fprintf(os.Stderr, "Terminal does not support inline images or protocol failed. Opening with default app...\n")
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", tmpFile.Name())
	case "linux":
		cmd = exec.Command("xdg-open", tmpFile.Name())
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", tmpFile.Name())
	default:
		fmt.Fprintf(os.Stderr, "Preview saved to %s\n", tmpFile.Name())
		return nil
	}

	// We don't remove the file here because the external application
	// needs it to stay on disk to display it.
	return cmd.Run()
}

// generatePreviewMultiple creates a visual diagram showing multiple paths and their IDs.
func generatePreviewMultiple(paths [][]mgl64.Vec3, ids []string, outputPath string) error {
	const (
		width  = 1200
		height = 900
		margin = 50
	)

	type Point2D struct {
		X, Y float64
	}

	project := func(p mgl64.Vec3) (float64, float64) {
		x := (p[0] - p[2]) * math.Cos(math.Pi/6)
		y := (p[0]+p[2])*math.Sin(math.Pi/6) - p[1]
		return x, y
	}

	// Calculate global bounds
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64

	var allPts2D [][]Point2D
	for _, path := range paths {
		var pts2d []Point2D
		for _, p := range path {
			u, v := project(p)
			pts2d = append(pts2d, Point2D{u, v})
			if u < minX {
				minX = u
			}
			if u > maxX {
				maxX = u
			}
			if v < minY {
				minY = v
			}
			if v > maxY {
				maxY = v
			}
		}
		allPts2D = append(allPts2D, pts2d)
	}

	if len(allPts2D) == 0 {
		return fmt.Errorf("no points to render")
	}

	dx := maxX - minX
	dy := maxY - minY
	if dx == 0 {
		dx = 1
	}
	if dy == 0 {
		dy = 1
	}

	scaleX := (width - 2*margin) / dx
	scaleY := (height - 2*margin) / dy
	scale := math.Min(scaleX, scaleY)

	offsetX := margin + (width-2*margin-dx*scale)/2 - minX*scale
	offsetY := margin + (height-2*margin-dy*scale)/2 - minY*scale

	transform := func(p Point2D) (float64, float64) {
		return p.X*scale + offsetX, p.Y*scale + offsetY
	}

	dc := gg.NewContext(width, height)

	// Try to load a clean modern font (Bold)
	fontPaths := []string{
		"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
		"/Library/Fonts/Arial Bold.ttf",
		"/System/Library/Fonts/Helvetica-Bold.otf",
		"/System/Library/Fonts/Helvetica-Bold.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
		"C:\\Windows\\Fonts\\arialbd.ttf",
		"/System/Library/Fonts/HelveticaNeue.ttc",
		"/System/Library/Fonts/Helvetica.ttc",
		"/System/Library/Fonts/SFNS.ttf",
	}
	fontLoaded := false
	for _, p := range fontPaths {
		if err := loadFont(dc, p, 24); err == nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Loaded bold font: %s\n", p)
			}
			fontLoaded = true
			break
		}
	}
	if !fontLoaded {
		// Fallback to regular fonts if bold not found
		regFontPaths := []string{
			"/System/Library/Fonts/Helvetica.ttc",
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"C:\\Windows\\Fonts\\arial.ttf",
		}
		for _, p := range regFontPaths {
			if err := loadFont(dc, p, 24); err == nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Loaded fallback font: %s\n", p)
				}
				fontLoaded = true
				break
			}
		}
	}
	if !fontLoaded {
		fmt.Fprintf(os.Stderr, "Warning: Could not load any of the preferred fonts, falling back to default.\n")
	}

	theme := getTheme()
	dc.SetColor(theme.Background)
	dc.Clear()

	// Draw grid/axes (optional)
	dc.SetColor(theme.Grid)
	dc.SetLineWidth(1)
	oU, oV := project(mgl64.Vec3{0, 0, 0})
	oX, oY := transform(Point2D{oU, oV})

	xU, xV := project(mgl64.Vec3{50, 0, 0})
	xX, xY := transform(Point2D{xU, xV})
	dc.DrawLine(oX, oY, xX, xY)
	dc.Stroke()
	dc.SetColor(theme.GridText)
	dc.DrawStringAnchored("X", xX, xY, 0.5, 0.5)
	dc.SetColor(theme.Grid)

	yU, yV := project(mgl64.Vec3{0, 50, 0})
	yX, yY := transform(Point2D{yU, yV})
	dc.DrawLine(oX, oY, yX, yY)
	dc.Stroke()
	dc.SetColor(theme.GridText)
	dc.DrawStringAnchored("Y", yX, yY, 0.5, 0.5)
	dc.SetColor(theme.Grid)

	zU, zV := project(mgl64.Vec3{0, 0, 50})
	zX, zY := transform(Point2D{zU, zV})
	dc.DrawLine(oX, oY, zX, zY)
	dc.Stroke()
	dc.SetColor(theme.GridText)
	dc.DrawStringAnchored("Z", zX, zY, 0.5, 0.5)

	// Draw paths
	colors := theme.Colors

	var drawnRects []labelRect

	// Helper to draw an annotated label with collision detection
	drawLabel := func(text string, baseX, baseY float64, preferredX, preferredY float64, c color.Color, minSegmentLen float64, currentSegmentLen float64) {
		if currentSegmentLen >= 0 && currentSegmentLen < minSegmentLen {
			return
		}

		lines := strings.Split(text, "\n")
		var sw, sh float64
		for _, line := range lines {
			w, h := dc.MeasureString(line)
			if w > sw {
				sw = w
			}
			sh += h
		}
		// Add some line spacing
		if len(lines) > 1 {
			sh += float64(len(lines)-1) * 6
		}

		pad := 6.0
		margin := 5.0

		bestX, bestY := baseX+preferredX, baseY+preferredY

		// Spiral search for non-overlapping position
		angle := 0.0
		radius := 0.0
		found := false
		for i := 0; i < 100; i++ {
			rect := labelRect{
				X: bestX - pad - margin,
				Y: bestY - sh - pad - margin,
				W: sw + (pad+margin)*2,
				H: sh + (pad+margin)*2,
			}

			overlap := false
			if rect.X < 0 || rect.Y < 0 || rect.X+rect.W > float64(width) || rect.Y+rect.H > float64(height) {
				overlap = true
			} else {
				for _, r := range drawnRects {
					if rect.Intersects(r) {
						overlap = true
						break
					}
				}
			}

			if !overlap {
				drawnRects = append(drawnRects, rect)
				found = true
				break
			}

			angle += 0.5
			radius += 5.0
			bestX = baseX + preferredX + math.Cos(angle)*radius
			bestY = baseY + preferredY + math.Sin(angle)*radius
		}

		if !found {
			return // skip if we can't find a spot
		}

		// Leader line if moved far from base point
		distFromBase := math.Sqrt(math.Pow(bestX-baseX, 2) + math.Pow(bestY-baseY, 2))
		if distFromBase > 30 {
			dc.SetColor(theme.LeaderLine)
			dc.SetDash() // Ensure leader line is solid
			dc.SetLineWidth(1)
			dc.DrawLine(baseX, baseY, bestX+sw/2, bestY-sh/2)
			dc.Stroke()
		}

		// Background
		dc.SetColor(theme.LabelBackground)
		dc.DrawRectangle(bestX-pad, bestY-sh-pad, sw+pad*2, sh+pad*2)
		dc.FillPreserve()
		dc.SetColor(theme.LabelBorder)
		dc.SetLineWidth(1)
		dc.Stroke()

		// Text
		dc.SetColor(c)
		currY := bestY - sh - 5
		for _, line := range lines {
			_, lh := dc.MeasureString(line)
			// Draw multiple times for fake bolding effect
			// Using sh/2 - (currY - (bestY - sh)) to find the relative offset
			dc.DrawStringAnchored(line, bestX+sw/2, currY+lh/2, 0.5, 0.5)
			dc.DrawStringAnchored(line, bestX+sw/2+0.5, currY+lh/2, 0.5, 0.5)
			dc.DrawStringAnchored(line, bestX+sw/2, currY+lh/2+0.5, 0.5, 0.5)
			currY += lh + 6 // line spacing
		}
	}

	for i, pts2d := range allPts2D {
		if len(pts2d) == 0 {
			continue
		}

		c := colors[i%len(colors)]

		// Draw annotations (bends and segment lengths) for this path
		wp, _ := calculateWirePath(paths[i], 1.0, 0.0, 0.0, "", false, false)

		// Map of segment index -> is part of an arc
		arcSegments := make(map[int]bool)
		var groups []bendGroup
		if wp != nil {
			groups = groupBends(wp.Bends)
			for _, g := range groups {
				if len(g.bends) > 1 {
					// Skip the first bend's index as it's the straight line leading into the arc
					for i := 1; i < len(g.bends); i++ {
						arcSegments[g.bends[i].Index] = true
					}
				}
			}
		}

		// Draw wire (distinguishing straight vs arc segments)
		for j := 0; j < len(pts2d)-1; j++ {
			x1, y1 := transform(pts2d[j])
			x2, y2 := transform(pts2d[j+1])

			if arcSegments[j] {
				r, g, b, a := c.RGBA()
				// Dimmer/lighter version of path color for arcs
				if strings.ToLower(previewTheme) == "cyberpunk" {
					dc.SetColor(color.RGBA{uint8(r / 256 / 2), uint8(g / 256 / 2), uint8(b / 256 / 2), uint8(a / 256)})
				} else {
					dc.SetColor(color.RGBA{uint8(r/512 + 150), uint8(g/512 + 150), uint8(b/512 + 150), uint8(a / 256)})
				}
				dc.SetLineWidth(2.5)
				dc.SetDash()
			} else {
				dc.SetColor(c)
				dc.SetLineWidth(5)
				dc.SetDash()
			}

			dc.DrawLine(x1, y1, x2, y2)
			dc.Stroke()
		}
		dc.SetDash()

		// Anchor at physical midpoint of the path
		totalLen := 0.0
		var segmentLens []float64
		for j := 0; j < len(paths[i])-1; j++ {
			l := paths[i][j].Sub(paths[i][j+1]).Len()
			totalLen += l
			segmentLens = append(segmentLens, l)
		}

		targetLen := totalLen / 2.0
		currentLen := 0.0
		var mid3D mgl64.Vec3
		if len(paths[i]) > 0 {
			mid3D = paths[i][0] // fallback
		}

		for j := 0; j < len(paths[i])-1; j++ {
			if currentLen+segmentLens[j] >= targetLen {
				remainder := targetLen - currentLen
				ratio := 0.0
				if segmentLens[j] > 0 {
					ratio = remainder / segmentLens[j]
				}
				p1, p2 := paths[i][j], paths[i][j+1]
				mid3D = mgl64.Vec3{
					p1[0] + (p2[0]-p1[0])*ratio,
					p1[1] + (p2[1]-p1[1])*ratio,
					p1[2] + (p2[2]-p1[2])*ratio,
				}
				break
			}
			currentLen += segmentLens[j]
		}

		midU, midV := project(mid3D)
		baseX, baseY := transform(Point2D{midU, midV})

		length := calculatePathLength(paths[i])
		labelStr := fmt.Sprintf("%s (%.1fmm)", ids[i], length)
		drawLabel(labelStr, baseX, baseY, 10, -10, c, 0, 0)

		if wp != nil {
			// Draw annotations
			for _, g := range groups {
				if len(g.bends) == 1 {
					// Single bend
					b := g.bends[0]
					if math.Abs(b.Angle) < 0.01 {
						continue
					}
					idx := b.Index + 1
					if idx >= len(pts2d) {
						continue
					}
					x, y := transform(pts2d[idx])

					label := fmt.Sprintf("%.1f°", b.Angle)
					if b.Rotation != 0 {
						label += fmt.Sprintf(" Rot:%.1f°", b.Rotation)
					}
					drawLabel(label, x, y, 15, -35, theme.BendLabel, 0, 0)
				} else {
					// Arc group
					totalAngle := 0.0
					var totalRadiusOfArc float64
					count := 0
					for _, b := range g.bends {
						totalAngle += b.Angle
						if math.Abs(b.Angle) > 0.01 {
							l := wp.Segments[b.Index].Len()
							angleRad := b.Angle * math.Pi / 180.0
							r := l / (2.0 * math.Sin(math.Abs(angleRad)/2.0))
							totalRadiusOfArc += r
							count++
						}
					}

					var radiusLabel string
					if count > 0 {
						avgRadius := totalRadiusOfArc / float64(count)
						radiusLabel = fmt.Sprintf(" (R:%.1fmm)", avgRadius)
					}

					var midX, midY float64
					// Find physical midpoint of the arc for the label
					midIdx := g.bends[len(g.bends)/2].Index + 1
					if midIdx < len(pts2d) {
						midX, midY = transform(pts2d[midIdx])
					}

					// Use the rotation of the first bend in the sequence
					rot := g.bends[0].Rotation
					label := fmt.Sprintf("Arc: %.1f°%s\n%d segments", totalAngle, radiusLabel, len(g.bends)+1)
					if rot != 0 {
						label += fmt.Sprintf("\nRot:%.1f°", rot)
					}
					drawLabel(label, midX, midY, 25, -50, theme.ArcLabel, 0, 0)
				}
			}

			// Draw segment lengths
			for k := 0; k < len(wp.Segments); k++ {
				if k+1 >= len(pts2d) {
					continue
				}
				x1, y1 := transform(pts2d[k])
				x2, y2 := transform(pts2d[k+1])

				midX, midY := (x1+x2)/2, (y1+y2)/2
				dist := wp.Segments[k].Len()
				// Only label segments > 10mm to avoid clutter in arcs
				drawLabel(fmt.Sprintf("%.1fmm", dist), midX, midY, 0, 45, theme.LengthLabel, 10.0, dist)
			}
		}
	}

	// Output
	if outputPath == "terminal" {
		var buf bytes.Buffer
		if err := dc.EncodePNG(&buf); err != nil {
			return err
		}
		return displayInTerminal(buf.Bytes())
	}

	if strings.HasSuffix(strings.ToLower(outputPath), ".svg") {
		return fmt.Errorf("SVG output not yet implemented; please use .png extension")
	}

	return dc.SavePNG(outputPath)
}
