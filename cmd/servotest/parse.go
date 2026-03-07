package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseM114 parses M114 response like:
//
//	FEED: 45.0 BEND: 0.0 ROTATE: 0.0
var m114Re = regexp.MustCompile(`(\w+):\s*([-\d.]+|ERROR\s*\([^)]*\))`)

func ParseM114(line string) (map[string]float64, error) {
	matches := m114Re.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no axis data found in: %q", line)
	}

	result := make(map[string]float64)
	for _, m := range matches {
		name := m[1]
		valStr := m[2]
		if strings.HasPrefix(valStr, "ERROR") {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing %s value %q: %w", name, valStr, err)
		}
		result[name] = val
	}
	return result, nil
}

// AxisStatus holds parsed M122 status for one axis.
type AxisStatus struct {
	Name    string
	ID      int
	Pos     float64
	Raw     int
	Speed   int
	Load    int
	Voltage int
	Temp    int
}

// ParseM122 parses M122 multi-line response. Each line like:
//
//	FEED: ID:1 Pos:45.0 Raw:3072 Speed:0 Load:0 Volt:7V Temp:35C
var m122Re = regexp.MustCompile(
	`(\w+):\s*ID:(\d+)\s+Pos:([-\d.]+)\s+Raw:([-\d]+)\s+Speed:([-\d]+)\s+Load:([-\d]+)\s+Volt:(\d+)V\s+Temp:(\d+)C`,
)

func ParseM122(lines []string) (map[string]*AxisStatus, error) {
	result := make(map[string]*AxisStatus)
	for _, line := range lines {
		m := m122Re.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		id, _ := strconv.Atoi(m[2])
		pos, _ := strconv.ParseFloat(m[3], 64)
		raw, _ := strconv.Atoi(m[4])
		speed, _ := strconv.Atoi(m[5])
		load, _ := strconv.Atoi(m[6])
		volt, _ := strconv.Atoi(m[7])
		temp, _ := strconv.Atoi(m[8])

		result[m[1]] = &AxisStatus{
			Name:    m[1],
			ID:      id,
			Pos:     pos,
			Raw:     raw,
			Speed:   speed,
			Load:    load,
			Voltage: volt,
			Temp:    temp,
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no axis status found in response")
	}
	return result, nil
}
