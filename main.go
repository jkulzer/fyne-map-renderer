package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	// "fyne.io/fyne/v2/dialog"

	"context"
	// "time"
	// "encoding/json"
	"fmt"
	"image/color"
	"os"

	"github.com/jkulzer/osm"
	"github.com/jkulzer/osm/osmpbf"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/project"

	"github.com/rs/zerolog/log"
)

type GeoJSON struct {
	Type       string      `json:"type"`
	Geometry   Geometry    `json:"geometry"`
	Properties interface{} `json:"properties"`
}

type Geometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"` // For LineString
}

func main() {
	a := app.New()
	w := a.NewWindow("Map Test")
	jsonFromFile, err := os.ReadFile("mapdata.geojson")
	if err != nil {
		panic(err)
	}

	parsedFC, _ := geojson.UnmarshalFeatureCollection(jsonFromFile)
	// parsedFC, _ := geojson.UnmarshalFeatureCollection(rawJSON)
	// _, _ = geojson.UnmarshalFeatureCollection(rawJSON)

	log.Info().Msg("finished processing of OSM data")

	// ============================================================

	// green := color.NRGBA{R: 0, G: 180, B: 0, A: 255}
	//
	// line1 := canvas.NewLine(color.White)
	//
	// line2 := canvas.NewLine(green)
	// line2.Move(fyne.NewPos(10, 10))
	// line2.Resize(fyne.NewSize(20, 20))
	// content := container.NewWithoutLayout(line1, line2)

	content := container.NewWithoutLayout()

	w.SetContent(content)

	// time.Sleep(time.Millisecond * 10)
	var lines []canvas.Line

	// contentSize := content.Size()

	var boundStartPoint orb.Point
	var boundEndPoint orb.Point
	boundStartPoint[1] = 52.678 // latitude
	boundStartPoint[0] = 13.079 // longitude

	boundEndPoint[1] = 52.337
	boundEndPoint[0] = 13.76

	projectedBoundStart := project.Point(boundStartPoint, project.WGS84.ToMercator)
	// projectedBoundEnd := project.Point(boundEndPoint, project.WGS84.ToMercator)

	latOffset := float32(projectedBoundStart[1])
	lonOffset := float32(projectedBoundStart[0])

	scaleFactor := float32(0.005)

	for _, feature := range parsedFC.Features {
		if feature.Geometry.GeoJSONType() == "LineString" {
			lineString := feature.Geometry.(orb.LineString)
			lsLastIndex := len(lineString) - 1
			for lsIndex, point := range lineString {
				if lsIndex != lsLastIndex {
					// latOffset := float32(52.337)

					endPoint := lineString[lsIndex+1]

					projectedStartPoint := project.Point(point, project.WGS84.ToMercator)
					projectedEndPoint := project.Point(endPoint, project.WGS84.ToMercator)

					// lonOffset := float32(13.76)

					startPointLat := float32(projectedStartPoint[1])
					startPointLon := float32(projectedStartPoint[0])
					endPointLat := float32(projectedEndPoint[1])
					endPointLon := float32(projectedEndPoint[0])

					// lineStartPosLat := (startPointLat - latOffset) * scaleFactor
					// lineStartPosLon := (lonOffset - startPointLon) * scaleFactor
					// lineEndPosLat := (endPointLat - latOffset) * scaleFactor
					// lineEndPosLon := (lonOffset - endPointLon) * scaleFactor
					lineStartPosLat := (latOffset - startPointLat) * scaleFactor
					lineStartPosLon := (startPointLon - lonOffset) * scaleFactor
					lineEndPosLat := (latOffset - endPointLat) * scaleFactor
					lineEndPosLon := (endPointLon - lonOffset) * scaleFactor

					line := canvas.NewLine(color.White)
					// line.Move(fyne.NewPos(lineStartPosLat, lineStartPosLon))
					// line.Resize(fyne.NewSize(lineEndPosLat-lineStartPosLat, lineEndPosLon-lineStartPosLon))
					line.Move(fyne.NewPos(lineStartPosLon, lineStartPosLat))
					line.Resize(fyne.NewSize(lineEndPosLon-lineStartPosLon, lineEndPosLat-lineStartPosLat))
					lines = append(lines, *line)
				}
			}
		}
	}

	fmt.Println(content.Visible())
	fmt.Println(content.Size())

	for _, line := range lines {
		content.Add(&line)
	}
	content.Refresh()
	// for _, line := range lines {
	// 	fmt.Println("============================================================================")
	// 	fmt.Println(line.Position1)
	// 	fmt.Println(line.Position2)
	// }
	// go func() {
	// 	time.Sleep(time.Second)
	//
	// 	line := canvas.NewLine(color.White)
	// 	// line.Move(fyne.NewPos(1, 1))
	// 	// line.Resize(fyne.NewSize(2, 2))
	// 	w.SetContent(line)
	// 	line.Position1 = fyne.NewPos(10, 10)
	// 	line.Position2 = fyne.NewPos(200, 200)
	// 	fmt.Println(line.Position1)
	// 	fmt.Println(line.Position2)
	// }()

	w.ShowAndRun()
}

func writeGeoJSON() {
	osmFile, err := os.Open("../fib-server/berlin-latest.osm.pbf")
	if err != nil {
		log.Err(err)
	}
	scanner := osmpbf.New(context.Background(), osmFile, 4)

	log.Info().Msg("starting processing of OSM data. this is blocking")

	nodes := make(map[osm.NodeID]*osm.Node)
	ways := make(map[osm.WayID]*osm.Way)
	relations := make(map[osm.RelationID]*osm.Relation)

	trainTracks := make(map[osm.WayID]*osm.Way)
	primaryHighways := make(map[osm.WayID]*osm.Way)

	for scanner.Scan() {
		// Get the next OSM object
		obj := scanner.Object()

		switch v := obj.(type) {
		case *osm.Node:
			nodes[v.ID] = v
		case *osm.Way:
			ways[v.ID] = v
			if v.Tags.Find("railway") == "light_rail" || v.Tags.Find("railway") == "subway" || v.Tags.Find("railway") == "rail" {
				trainTracks[v.ID] = v
			}
			if v.Tags.Find("highway") == "primary" {
				primaryHighways[v.ID] = v
			}
		case *osm.Relation:
			relations[v.ID] = v
		default:
			// Handle other OSM object types if needed
		}
	}

	fmt.Println(len(trainTracks))
	fc := geojson.NewFeatureCollection()
	for _, way := range trainTracks {
		var lineString orb.LineString
		for _, wayNode := range way.Nodes {
			point := nodes[wayNode.ID].Point()
			lineString = append(lineString, point)
		}
		feature := geojson.NewFeature(lineString)
		fc.Append(feature)
	}
	// for _, way := range primaryHighways {
	// 	var lineString orb.LineString
	// 	for _, wayNode := range way.Nodes {
	// 		point := nodes[wayNode.ID].Point()
	// 		lineString = append(lineString, point)
	// 	}
	// 	feature := geojson.NewFeature(lineString)
	// 	fc.Append(feature)
	// }
	rawJSON, _ := fc.MarshalJSON()

	f, err := os.Create("mapdata.geojson")

	defer f.Close()
	_, err = f.Write(rawJSON)
	if err != nil {
		log.Err(err).Msg("")
	}
}
