package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	// "fyne.io/fyne/v2/dialog"

	"context"
	"time"
	// "encoding/json"
	"fmt"
	"image/color"
	"os"

	"github.com/jkulzer/osm"
	"github.com/jkulzer/osm/osmpbf"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"

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

	go func() {
		time.Sleep(time.Second)
		var lines []canvas.Line

		// contentSize := content.Size()

		for _, feature := range parsedFC.Features {
			fmt.Println(feature.Type)
			if feature.Geometry.GeoJSONType() == "LineString" {
				lineString := feature.Geometry.(orb.LineString)
				lsLastIndex := len(lineString) - 1
				for lsIndex, point := range lineString {
					if lsIndex != lsLastIndex {
						// latOffset := float32(52.337)

						latOffset := float32(52.678)
						lonOffset := float32(13.079)

						// lonOffset := float32(13.76)

						startPointLat := float32(point[1])
						startPointLon := float32(point[0])
						endPoint := lineString[lsIndex+1]
						endPointLat := float32(endPoint[1])
						endPointLon := float32(endPoint[0])

						fmt.Println("==========================")

						scaleFactor := float32(300)

						// lineStartPosLat := (startPointLat - latOffset) * scaleFactor
						// lineStartPosLon := (lonOffset - startPointLon) * scaleFactor
						// lineEndPosLat := (endPointLat - latOffset) * scaleFactor
						// lineEndPosLon := (lonOffset - endPointLon) * scaleFactor
						lineStartPosLat := (latOffset - startPointLat) * scaleFactor
						lineStartPosLon := (startPointLon - lonOffset) * scaleFactor
						lineEndPosLat := (latOffset - endPointLat) * scaleFactor
						lineEndPosLon := (endPointLon - lonOffset) * scaleFactor

						fmt.Println(lineStartPosLat)
						fmt.Println(lineStartPosLon)

						fmt.Println(lineEndPosLat)
						fmt.Println(lineEndPosLon)

						line := canvas.NewLine(color.White)
						// line.Move(fyne.NewPos(lineStartPosLat, lineStartPosLon))
						// line.Resize(fyne.NewSize(lineEndPosLat-lineStartPosLat, lineEndPosLon-lineStartPosLon))
						line.Move(fyne.NewPos(lineStartPosLon, lineStartPosLat))
						line.Resize(fyne.NewSize(lineEndPosLon-lineStartPosLon, lineEndPosLat-lineStartPosLat))
						lines = append(lines, *line)
						fmt.Println(line.Position())
						fmt.Println(line.Size())
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
	}()
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
