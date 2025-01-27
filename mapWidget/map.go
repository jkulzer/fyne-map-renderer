package mapWidget

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"net/http"
	"net/url"

	"github.com/nfnt/resize"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	// "github.com/paulmach/orb/project"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"golang.org/x/image/draw"
)

const tileSize = 256

// Map widget renders an interactive map using OpenStreetMap tile data.
type Map struct {
	widget.BaseWidget

	pixels       *image.NRGBA
	w, h         int
	zoom, x, y   int
	dragX, dragY float32

	cl *http.Client

	tileSource       string // url to download xyz tiles (example: "https://tile.openstreetmap.org/%d/%d/%d.png")
	hideAttribution  bool   // enable copyright attribution
	attributionLabel string // label for attribution (example: "OpenStreetMap")
	attributionURL   string // url for attribution (example: "https://openstreetmap.org")
	hideZoomButtons  bool   // enable zoom buttons
	hideMoveButtons  bool   // enable move map buttons

	lineColor color.Color

	featureCollection *geojson.FeatureCollection // overlay to render
}

// MapOption configures the provided map with different features.
type MapOption func(*Map)

// WithOsmTiles configures the map to use osm tile source.
func WithOsmTiles() MapOption {
	return func(m *Map) {
		// m.tileSource = "https://tile.openstreetmap.org/%d/%d/%d.png"
		m.tileSource = "https://b.tile.openstreetmap.fr/hot/%d/%d/%d.png"
		m.attributionLabel = "OpenStreetMap"
		m.attributionURL = "https://openstreetmap.org"
		m.hideAttribution = false
	}
}

// WithTileSource configures the map to use a custom tile source.
func WithTileSource(tileSource string) MapOption {
	return func(m *Map) {
		m.tileSource = tileSource
	}
}

// WithAttribution configures the map widget to display an attribution.
func WithAttribution(enable bool, label, url string) MapOption {
	return func(m *Map) {
		m.hideAttribution = !enable
		m.attributionLabel = label
		m.attributionURL = url
	}
}

// WithZoomButtons enables or disables zoom controls.
func WithZoomButtons(enable bool) MapOption {
	return func(m *Map) {
		m.hideZoomButtons = !enable
	}
}

// WithScrollButtons enables or disables map scroll controls.
func WithScrollButtons(enable bool) MapOption {
	return func(m *Map) {
		m.hideMoveButtons = !enable
	}
}

// WithHTTPClient configures the map to use a custom http client.
func WithHTTPClient(client *http.Client) MapOption {
	return func(m *Map) {
		m.cl = client
	}
}

// NewMap creates a new instance of the map widget.
func NewMap(fc *geojson.FeatureCollection) *Map {
	m := &Map{cl: &http.Client{}}
	WithOsmTiles()(m)

	m.lineColor = color.RGBA{
		R: 245,
		G: 239,
		B: 31,
		A: 255,
	}
	m.zoom = 10
	m.x = 38
	m.y = -176
	m.featureCollection = fc
	// fmt.Println(tileToDegree(x, y, m.zoom))
	m.ExtendBaseWidget(m)
	return m
}

// NewMapWithOptions creates a new instance of the map widget with provided map options.
func NewMapWithOptions(fc *geojson.FeatureCollection, opts ...MapOption) *Map {
	m := NewMap(fc)
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// MinSize returns the smallest possible size for a widget.
// For our map this is a constant size representing a single tile on a device with
// the highest known DPI (4x).
func (m *Map) MinSize() fyne.Size {
	return fyne.NewSize(64, 64)
}

// PanEast will move the map to the East by 1 tile.
func (m *Map) PanEast() {
	m.x++
	m.Refresh()
}

// PanNorth will move the map to the North by 1 tile.
func (m *Map) PanNorth() {
	m.y--
	m.Refresh()
}

// PanSouth will move the map to the South by 1 tile.
func (m *Map) PanSouth() {
	m.y++
	m.Refresh()
}

// PanWest will move the map to the west by 1 tile.
func (m *Map) PanWest() {
	m.x--
	m.Refresh()
}

// Zoom sets the zoom level to a specific value, between 0 and 19.
func (m *Map) Zoom(zoom int) {
	if zoom < 10 || zoom > 19 {
		return
	}
	delta := zoom - m.zoom
	if delta > 0 {
		for i := 0; i < delta; i++ {
			m.zoomInStep()
		}
	} else if delta < 0 {
		for i := 0; i > delta; i-- {
			m.zoomOutStep()
		}
	}
	m.Refresh()
}

// ZoomIn steps the scale of this map to be one step zoomed in.
func (m *Map) ZoomIn() {
	if m.zoom >= 19 {
		return
	}
	m.zoomInStep()
	m.Refresh()
}

// ZoomOut steps the scale of this map to be one step zoomed out.
func (m *Map) ZoomOut() {
	if m.zoom <= 0 {
		return
	}
	m.zoomOutStep()
	m.Refresh()
}

// CreateRenderer returns the renderer for this widget.
// A map renderer is simply the map Raster with user interface elements overlaid.
func (m *Map) CreateRenderer() fyne.WidgetRenderer {
	fmt.Println(m.x)
	fmt.Println(m.y)

	var zoom fyne.CanvasObject
	if !m.hideZoomButtons {
		zoom = container.NewVBox(
			newMapButton(theme.ZoomInIcon(), m.ZoomIn),
			newMapButton(theme.ZoomOutIcon(), m.ZoomOut))
	}

	var move fyne.CanvasObject
	if !m.hideMoveButtons {
		buttonLayout := container.NewGridWithColumns(3, layout.NewSpacer(),
			newMapButton(theme.MoveUpIcon(), m.PanNorth), layout.NewSpacer(),
			newMapButton(theme.NavigateBackIcon(), m.PanWest), layout.NewSpacer(),
			newMapButton(theme.NavigateNextIcon(), m.PanEast), layout.NewSpacer(),
			newMapButton(theme.MoveDownIcon(), m.PanSouth), layout.NewSpacer())
		move = container.NewVBox(buttonLayout)
	}

	var copyright fyne.CanvasObject
	if !m.hideAttribution {
		license, _ := url.Parse(m.attributionURL)
		link := widget.NewHyperlink(m.attributionLabel, license)
		copyright = container.NewHBox(layout.NewSpacer(), link)
	}

	overlay := container.NewBorder(nil, copyright, move, zoom)

	customOverlay := m.geoJSONOverlay()

	c := container.NewStack(canvas.NewRaster(m.draw), customOverlay, container.NewPadded(overlay))
	// c := customOverlay
	return widget.NewSimpleRenderer(c)
}

func (m *Map) draw(w, h int) image.Image {
	scale := 1
	tileSize := tileSize
	// TODO use retina tiles once OSM supports it in their server (text scaling issues)...
	if c := fyne.CurrentApp().Driver().CanvasForObject(m); c != nil {
		scale = int(c.Scale())
		if scale < 1 {
			scale = 1
		}
		tileSize = tileSize * scale
	}

	if m.w != w || m.h != h {
		m.pixels = image.NewNRGBA(image.Rect(0, 0, w, h))
	}

	midTileX := (w - tileSize*2) / 2
	midTileY := (h - tileSize*2) / 2
	if m.zoom == 0 {
		midTileX += tileSize / 2
		midTileY += tileSize / 2
	}

	count := 1 << m.zoom
	mx := m.x + int(float32(count)/2-0.5)
	my := m.y + int(float32(count)/2-0.5)
	firstTileX := mx - int(math.Ceil(float64(midTileX)/float64(tileSize)))
	firstTileY := my - int(math.Ceil(float64(midTileY)/float64(tileSize)))

	for x := firstTileX; (x-firstTileX)*tileSize <= w+tileSize; x++ {
		for y := firstTileY; (y-firstTileY)*tileSize <= h+tileSize; y++ {
			if x < 0 || y < 0 || x >= int(count) || y >= int(count) {
				continue
			}

			src, err := getTile(m.tileSource, x, y, m.zoom, m.cl)
			if err != nil {
				fyne.LogError("tile fetch error", err)
				continue
			}

			pos := image.Pt(midTileX+(x-mx)*tileSize,
				midTileY+(y-my)*tileSize)
			scaled := src
			if scale > 1 {
				scaled = resize.Resize(uint(tileSize), uint(tileSize), src, resize.Lanczos2)
			}
			draw.Copy(m.pixels, pos, scaled, image.Rect(0, 0, tileSize, tileSize), draw.Over, nil)
		}
	}

	return m.pixels
}

func (m *Map) geoJSONOverlay() *fyne.Container {
	fmt.Println("=====================================")
	x, y := XYToTile(m.x, m.y, m.zoom)
	// x := float64(m.x) + ((math.Pow(float64(2), float64(m.zoom))) / 2)
	// y := float64(m.y) + ((math.Pow(float64(2), float64(m.zoom))) / 2)
	fmt.Println("absolute tile x: " + fmt.Sprint(x))
	fmt.Println("absolute tile y: " + fmt.Sprint(y))
	lat, lon := TileToCoords(int(x), int(y), m.zoom)
	lonLeft, latTop := TileToCoords(int(x)-2, int(y)-1, m.zoom)
	lonRight, latBottom := TileToCoords(int(x)+2, int(y)+1, m.zoom)

	var leftTopPoint orb.Point
	leftTopPoint[1] = latTop
	leftTopPoint[0] = lonLeft
	var rightBottomPoint orb.Point
	rightBottomPoint[1] = latBottom
	rightBottomPoint[0] = lonRight

	size := m.Size()

	fmt.Println(size)
	fmt.Println("lat: " + fmt.Sprint(lat))
	fmt.Println("lon: " + fmt.Sprint(lon))
	fmt.Println("lat left: " + fmt.Sprint(latTop))
	fmt.Println("lon left: " + fmt.Sprint(lonLeft))
	fmt.Println("lat right: " + fmt.Sprint(latBottom))
	fmt.Println("lon right: " + fmt.Sprint(lonRight))

	var lines []canvas.Line

	for _, feature := range m.featureCollection.Features {
		if feature.Geometry.GeoJSONType() == "LineString" {
			lineString := feature.Geometry.(orb.LineString)
			lsLastIndex := len(lineString) - 1
			for lsIndex, point := range lineString {
				if lsIndex != lsLastIndex {
					// latOffset := float32(52.337)

					endPoint := lineString[lsIndex+1]

					fmt.Println("start point: " + fmt.Sprint(point))
					fmt.Println("end point: " + fmt.Sprint(endPoint))

					fmt.Println("leftTop: " + fmt.Sprint(leftTopPoint))
					fmt.Println("rightBottom: " + fmt.Sprint(rightBottomPoint))

					startLatDiff := leftTopPoint[1] - point[1]
					startLonDiff := point[0] - leftTopPoint[0]

					endLatDiff := leftTopPoint[1] - endPoint[1]
					endLonDiff := endPoint[0] - leftTopPoint[0]

					coordWidth := lonRight - lonLeft
					coordHeight := latTop - latBottom

					fmt.Println("viewport coords width: " + fmt.Sprint(coordWidth))
					fmt.Println("viewport coords height: " + fmt.Sprint(coordHeight))

					scaleConst := float32(10)
					startX := float32(startLonDiff*coordHeight) * size.Width * scaleConst * 2
					startY := float32(startLatDiff*coordWidth) * size.Height * scaleConst
					endX := float32(endLonDiff*coordHeight) * size.Width * scaleConst * 2
					endY := float32(endLatDiff*coordWidth) * size.Height * scaleConst

					fmt.Println(startX)
					fmt.Println(startY)
					fmt.Println(endX)
					fmt.Println(endY)

					line := canvas.NewLine(m.lineColor)

					line.Move(fyne.NewPos(startX, startY))
					lineSize := fyne.NewSize(endX-startX, endY-startY)
					line.Resize(lineSize)

					fmt.Println("=======================================")

					switch feature.Properties["category"] {
					case "subway":
						line.StrokeColor = m.lineColor
						line.StrokeWidth = 3
					case "light_rail":
						line.StrokeWidth = 3
						line.StrokeColor = m.lineColor
					case "water":
						line.StrokeWidth = 1
						line.StrokeColor = m.lineColor
					case "primary_highway":
						line.StrokeWidth = 2
						line.StrokeColor = m.lineColor
					case "secondary_highway":
						line.StrokeWidth = 1
						line.StrokeColor = m.lineColor
					case "tertiary_highway":
						line.StrokeWidth = 1
						line.StrokeColor = m.lineColor
					}

					lines = append(lines, *line)
				}
			}
		}
	}

	// var canvas fyne.Canvas
	mapCanvas := container.NewWithoutLayout()
	fmt.Println("test")

	for _, line := range lines {
		mapCanvas.Add(&line)
		// fmt.Println(line)
		// fmt.Println(leftTopProjected)
		// fmt.Println(rightBottomProjected)
	}
	return mapCanvas
}

func (m *Map) zoomInStep() {
	m.zoom++
	m.x *= 2
	m.y *= 2
}

func (m *Map) zoomOutStep() {
	m.zoom--
	m.x /= 2
	m.y /= 2
}

func (m *Map) SetPosition(x, y int) {
	m.x = x
	m.y = y
}
func (m *Map) GetPosition() (int, int) {
	return m.x, m.y
}

func (m *Map) GetZoom() int {
	return m.zoom
}
