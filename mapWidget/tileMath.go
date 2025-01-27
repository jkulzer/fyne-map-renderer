package mapWidget

import (
	"math"
)

// source: https://www.netzwolf.info/geo/math/tilebrowser.html?tx=550&ty=335&tz=10#tile

func XYToTile(x, y, zoom int) (float64, float64) {
	tileX := float64(x) + ((math.Pow(float64(2), float64(zoom))) / 2)
	tileY := float64(y) + ((math.Pow(float64(2), float64(zoom))) / 2)
	return tileX, tileY
}

func TileToCoords(tileX, tileY, zoom int) (float64, float64) {
	x := (float64(tileX)) / (math.Pow(2.0, float64(zoom)))
	y := (float64(tileY)) / (math.Pow(2.0, float64(zoom)))
	mercatorL := +(x*2 - 1) * math.Pi
	mercatorW := -(y*2 - 1) * math.Pi
	length := mercatorL
	width := 2*math.Atan(math.Exp(mercatorW)) - (math.Pi / 2)
	lat := length / math.Pi * 180
	lon := width / math.Pi * 180
	return lat, lon
}
