package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

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
	"github.com/paulmach/orb/project"
	"github.com/paulmach/orb/simplify"

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

type MapWidget struct {
	widget.BaseWidget
	content                 *fyne.Container
	boundStartPoint         orb.Point
	boundEndPoint           orb.Point
	projectedBoundStart     orb.Point
	latOffset               float32
	lonOffset               float32
	scaleFactor             float32
	parsedFeatureCollection *geojson.FeatureCollection
	lines                   []canvas.Line
	grey                    color.RGBA
	yellow                  color.RGBA
	blue                    color.RGBA
}

func NewMapWidget(parsedFeatureCollection *geojson.FeatureCollection) *MapWidget {
	w := &MapWidget{}
	w.ExtendBaseWidget(w)

	w.parsedFeatureCollection = parsedFeatureCollection

	w.content = container.NewWithoutLayout()

	w.yellow = color.RGBA{
		R: 255,
		G: 236,
		B: 66,
		A: 255,
	}
	w.grey = color.RGBA{
		R: 150,
		G: 150,
		B: 150,
		A: 255,
	}
	w.blue = color.RGBA{
		R: 100,
		G: 218,
		B: 250,
		A: 255,
	}
	fmt.Println(w.parsedFeatureCollection.BBox)

	// contentSize := content.Size()
	w.boundStartPoint[1] = 52.678 // latitude
	w.boundStartPoint[0] = 13.079 // longitude

	w.boundEndPoint[1] = 52.337
	w.boundEndPoint[0] = 13.76

	w.projectedBoundStart = project.Point(w.boundStartPoint, project.WGS84.ToMercator)
	// projectedBoundEnd := project.Point(boundEndPoint, project.WGS84.ToMercator)

	w.latOffset = float32(w.projectedBoundStart[1])
	w.lonOffset = float32(w.projectedBoundStart[0])

	w.scaleFactor = float32(0.005)

	// w.content = container.NewVBox(container.NewScroll(w.content))
	go func() {
		time.Sleep(time.Second)
		w.Refresh()

	}()

	return w
}

func (w *MapWidget) CreateRenderer() fyne.WidgetRenderer {
	w.content = container.NewWithoutLayout()

	rect := canvas.NewRectangle(w.grey)
	widgetSize := w.content.Size()
	rect.FillColor = w.grey
	rect.Move(fyne.NewPos(0, 0))
	rect.Resize(widgetSize)

	for _, feature := range w.parsedFeatureCollection.Features {
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

					lineStartPosLat := (w.latOffset - startPointLat) * w.scaleFactor
					lineStartPosLon := (startPointLon - w.lonOffset) * w.scaleFactor
					lineEndPosLat := (w.latOffset - endPointLat) * w.scaleFactor
					lineEndPosLon := (endPointLon - w.lonOffset) * w.scaleFactor

					line := canvas.NewLine(w.grey)

					line.Move(fyne.NewPos(lineStartPosLon, lineStartPosLat))
					line.Resize(fyne.NewSize(lineEndPosLon-lineStartPosLon, lineEndPosLat-lineStartPosLat))

					switch feature.Properties["category"] {
					case "subway":
						line.StrokeColor = w.yellow
						line.StrokeWidth = 3
					case "light_rail":
						line.StrokeWidth = 3
						line.StrokeColor = w.yellow
					case "water":
						line.StrokeWidth = 1
						line.StrokeColor = w.blue
					case "primary_highway":
						line.StrokeWidth = 2
						line.StrokeColor = w.grey
					case "secondary_highway":
						line.StrokeWidth = 1
						line.StrokeColor = w.grey
					case "tertiary_highway":
						line.StrokeWidth = 1
						line.StrokeColor = w.grey
					}

					w.lines = append(w.lines, *line)
				}
			}
		}
	}
	fmt.Println("test")

	for _, line := range w.lines {
		w.content.Add(&line)
	}

	w.content.Refresh()
	return widget.NewSimpleRenderer(w.content)
}

func main() {
	a := app.New()
	w := a.NewWindow("Map Test")

	writeGeoJSON()

	// jsonFromFile, err := os.ReadFile("simple.geojson")
	jsonFromFile, err := os.ReadFile("mapdata.geojson")
	if err != nil {
		panic(err)
	}

	parsedFC, _ := geojson.UnmarshalFeatureCollection(jsonFromFile)
	// parsedFC, _ := geojson.UnmarshalFeatureCollection(rawJSON)
	// _, _ = geojson.UnmarshalFeatureCollection(rawJSON)

	w.SetContent(NewMapWidget(parsedFC))

	log.Info().Msg("finished processing of OSM data")

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

	// subwayWays := make(map[osm.WayID]*osm.Way)
	// sbahnWays := make(map[osm.WayID]*osm.Way)

	// trainTracks := make(map[osm.WayID]*osm.Way)
	primaryHighways := make(map[osm.WayID]*osm.Way)
	secondaryHighways := make(map[osm.WayID]*osm.Way)
	tertiaryHighways := make(map[osm.WayID]*osm.Way)
	rivers := make(map[osm.WayID]*osm.Way)

	subwayLines := make(map[osm.RelationID]*osm.Relation)
	sbahnLines := make(map[osm.RelationID]*osm.Relation)

	fc := geojson.NewFeatureCollection()

	for scanner.Scan() {
		// Get the next OSM object
		obj := scanner.Object()

		switch v := obj.(type) {
		case *osm.Node:
			nodes[v.ID] = v
		case *osm.Way:
			ways[v.ID] = v
			if v.Tags.Find("waterway") == "river" {
				rivers[v.ID] = v
			}
			highwayTag := v.Tags.Find("highway")
			if highwayTag == "primary" {
				primaryHighways[v.ID] = v
			} else if highwayTag == "secondary" {
				secondaryHighways[v.ID] = v
			} else if highwayTag == "tertiary" {
				tertiaryHighways[v.ID] = v
			}
		case *osm.Relation:
			relations[v.ID] = v
			routeTag := v.Tags.Find("route")
			if routeTag == "subway" {
				subwayLines[v.ID] = v
			} else if routeTag == "light_rail" {
				sbahnLines[v.ID] = v
			}

		default:
			// Handle other OSM object types if needed
		}
	}

	var riverCollection orb.Collection
	var tertiaryHighwayCollection orb.Collection
	var secondaryHighwayCollection orb.Collection
	var primaryHighwayCollection orb.Collection
	var subwayCollection orb.Collection
	var lightRailCollection orb.Collection

	for _, river := range rivers {
		addToFeatureCollection(river, fc, "water", nodes, &riverCollection)
	}
	for _, road := range tertiaryHighways {
		addToFeatureCollection(road, fc, "tertiary_highway", nodes, &tertiaryHighwayCollection)
	}
	for _, road := range secondaryHighways {
		addToFeatureCollection(road, fc, "secondary_highway", nodes, &secondaryHighwayCollection)
	}
	for _, road := range primaryHighways {
		addToFeatureCollection(road, fc, "primary_highway", nodes, &primaryHighwayCollection)
	}
	simplifyWaysAndAdd(primaryHighways, fc, "primary_highway")
	for _, subwayLine := range subwayLines {
		for _, member := range subwayLine.Members {
			if member.Type == "way" {
				wayID, err := member.ElementID().WayID()
				if err != nil {
					log.Err(err).Msg("")
				}
				memberWay := ways[wayID]
				addToFeatureCollection(memberWay, fc, "subway", nodes, &subwayCollection)
			}
		}
	}
	for _, sbahnLine := range sbahnLines {
		for _, member := range sbahnLine.Members {
			if member.Type == "way" {
				wayID, err := member.ElementID().WayID()
				if err != nil {
					log.Err(err).Msg("")
				}
				memberWay := ways[wayID]
				addToFeatureCollection(memberWay, fc, "light_rail", nodes, &lightRailCollection)
			}
		}
	}

	// simplifyConst := 0.001
	simplifyConst := 0.00001
	simplify.DouglasPeucker(simplifyConst).Collection(riverCollection)
	simplify.DouglasPeucker(simplifyConst).Collection(tertiaryHighwayCollection)
	simplify.DouglasPeucker(simplifyConst).Collection(secondaryHighwayCollection)
	simplify.DouglasPeucker(simplifyConst).Collection(primaryHighwayCollection)
	simplify.DouglasPeucker(simplifyConst).Collection(subwayCollection)
	simplify.DouglasPeucker(simplifyConst).Collection(lightRailCollection)

	featCol := geojson.NewFeatureCollection()
	appendToFeatureCollection(riverCollection, featCol, "water")
	appendToFeatureCollection(tertiaryHighwayCollection, featCol, "tertiary_highway")
	appendToFeatureCollection(secondaryHighwayCollection, featCol, "secondary_highway")
	appendToFeatureCollection(primaryHighwayCollection, featCol, "primary_highway")
	appendToFeatureCollection(subwayCollection, featCol, "subway")
	appendToFeatureCollection(lightRailCollection, featCol, "light_rail")

	// rawJSON, _ := fc.MarshalJSON()
	rawJSON, _ := featCol.MarshalJSON()
	fmt.Println(len(rawJSON))

	f, err := os.Create("mapdata.geojson")

	defer f.Close()
	_, err = f.Write(rawJSON)
	if err != nil {
		log.Err(err).Msg("")
	}
}

func simplifyWaysAndAdd(ways map[osm.WayID]*osm.Way, fc *geojson.FeatureCollection, category string) {
	var wayArray []*osm.Way
	for _, way := range ways {
		wayArray = append(wayArray, way)
	}
	wayArrLen := len(wayArray)
	for wayIndex, way := range wayArray {
		if wayIndex+1 < wayArrLen {
			if way.Nodes[len(way.Nodes)-1] == wayArray[wayIndex+1].Nodes[0] {
				fmt.Println("got it")
			}
		}
	}
}

func addToFeatureCollection(way *osm.Way, fc *geojson.FeatureCollection, category string, nodes map[osm.NodeID]*osm.Node, collection *orb.Collection) {
	var lineString orb.LineString
	if way != nil {
		for _, wayNode := range way.Nodes {
			point := nodes[wayNode.ID].Point()
			lineString = append(lineString, point)
		}
		threshold := 0.0000001
		simplify.DouglasPeucker(threshold).Simplify(lineString)
		feature := geojson.NewFeature(lineString)
		feature.Properties["category"] = category
		fc.Append(feature)
		*collection = append(*collection, lineString)
	}
}

func appendToFeatureCollection(collection orb.Collection, featCol *geojson.FeatureCollection, category string) {
	for _, item := range collection {
		feature := geojson.NewFeature(item)
		feature.Properties["category"] = category
		featCol.Append(feature)
	}
}
