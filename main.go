package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jkulzer/osm"
	"github.com/jkulzer/osm/osmpbf"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/simplify"

	"github.com/rs/zerolog/log"
	// "time"

	"fyne.io/fyne/v2/app"
	// "fyne.io/fyne/v2/container"
	"github.com/jkulzer/fyne-map-renderer/mapWidget"
)

func main() {
	a := app.New()
	w := a.NewWindow("Draggable Map Example")

	writeGeoJSON()

	jsonFromFile, err := os.ReadFile("mapdata.geojson")
	if err != nil {
		panic(err)
	}

	parsedFC, _ := geojson.UnmarshalFeatureCollection(jsonFromFile)

	widget := mapWidget.NewMap(parsedFC)
	// widget.Zoom(10)
	// widget.SetPosition(38, -176)
	// 10
	// 38 - 176
	w.SetContent(widget)

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

	// appendToFeatureCollection(riverCollection, featCol, "water")
	// appendToFeatureCollection(tertiaryHighwayCollection, featCol, "tertiary_highway")
	// appendToFeatureCollection(secondaryHighwayCollection, featCol, "secondary_highway")
	// appendToFeatureCollection(primaryHighwayCollection, featCol, "primary_highway")
	// appendToFeatureCollection(subwayCollection, featCol, "subway")
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
