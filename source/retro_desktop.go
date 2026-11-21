//go:build desktop

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"reflect"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	retroBlack          = color.NRGBA{0, 0, 0, 255}
	retroYellow         = color.NRGBA{255, 245, 0, 255}
	retroWhite          = color.NRGBA{235, 237, 240, 255}
	retroGreen          = color.NRGBA{28, 255, 60, 255}
	retroMuted          = color.NRGBA{118, 124, 138, 255}
	retroBlue           = color.NRGBA{0, 20, 150, 255}
	retroEliminationRed = color.NRGBA{105, 12, 22, 255}
	retroNavy           = color.NRGBA{10, 14, 40, 255}
)

type retroTheme struct{ fyne.Theme }

func (t retroTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return retroBlack
	case theme.ColorNameForeground:
		return retroWhite
	case theme.ColorNamePrimary:
		return retroYellow
	case theme.ColorNameButton:
		return retroNavy
	}
	return t.Theme.Color(n, theme.VariantDark)
}
func retroText(text string, colour color.Color, size float32) *canvas.Text {
	t := canvas.NewText(text, colour)
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	t.TextSize = size
	return t
}

type retroTap struct {
	widget.BaseWidget
	content fyne.CanvasObject
	tapped  func()
}

func tapRetro(content fyne.CanvasObject, fn func()) *retroTap {
	t := &retroTap{content: content, tapped: fn}
	t.ExtendBaseWidget(t)
	return t
}
func (t *retroTap) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }
func (t *retroTap) Tapped(*fyne.PointEvent)             { t.tapped() }

// One aligned row of canvas objects. Every row shares the same column weights,
// and text scales with the available width rather than wrapping into more rows.
type retroRowLayout struct {
	columns []retroColumn
	header  bool
}

func (l retroRowLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(retroColumnsMinWidth(l.columns), 25)
}
func (l retroRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	widths := retroColumnWidths(l.columns, size.Width)
	x := float32(8)
	font := float32(14)
	if size.Width < 1450 {
		font = 12
	}
	if l.header {
		font = 12
	}
	for i, obj := range objects {
		if i >= len(l.columns) {
			break
		}
		w := widths[i]
		if fitted, ok := obj.(*retroFittedText); ok {
			if fitted.Text.TextSize != font {
				fitted.Text.TextSize = font
				fitted.fit()
			}
		}
		if t, ok := obj.(*canvas.Text); ok {
			if t.TextSize != font {
				t.TextSize = font
				t.Refresh()
			}
		}
		obj.Move(fyne.NewPos(x, (size.Height-22)/2))
		obj.Resize(fyne.NewSize(w-5, 22))
		x += w
	}
}

// The canvas tree is retained between samples. A cell owns its canvas objects;
// changing a value never creates a replacement row, control or image texture.
type retroCellView struct {
	label *retroFittedText
	text  *canvas.Text
	image *canvas.Image
	node  fyne.CanvasObject
	png   string
}
type retroRowView struct {
	node       *fyne.Container
	background *canvas.Rectangle
	cells      []retroCellView
}
type retroScreen struct {
	preferenceKey                                string
	handles                                      map[string]*retroColumnHandle
	draggingWidth, pendingWidth                  bool
	pendingKey                                   string
	pendingValue                                 float32
	widthGeneration                              uint64
	slides                                       *retroSlideLayout
	bridge                                       *desktopBridge
	images                                       map[string]fyne.Resource
	root                                         fyne.CanvasObject
	layout                                       sessionLayout
	columns                                      []retroColumn
	columnsOpen, hasMessage, hasError, hasSetup  bool
	setupCount                                   int
	state, subtitle, status, message, setupTitle *canvas.Text
	errorLabel                                   *widget.Label
	setupValues                                  []*canvas.Text
	checks                                       map[string]*widget.Check
	body                                         *fyne.Container
	rows                                         map[string]*retroRowView
	order                                        []string
	headings                                     fyne.CanvasObject
	separators                                   []*canvas.Rectangle
}

func setRetroText(text *canvas.Text, value string, colour color.Color) {
	if text.Text != value || text.Color != colour {
		text.Text = value
		text.Color = colour
		text.Refresh()
	}
}
func retroCellColour(key string, cell retroCell, retired bool) color.Color {
	if retired {
		return retroMuted
	}
	switch cell.Tone {
	case "good":
		return retroGreen
	case "warn":
		return retroYellow
	case "bad":
		return color.NRGBA{255, 65, 65, 255}
	case "muted":
		return retroMuted
	case "fastest":
		return color.NRGBA{255, 0, 255, 255}
	}
	switch key {
	case "driver":
		return retroYellow
	case "FL", "FR", "RL", "RR", "Avg":
		return retroGreen
	}
	return retroWhite
}
func (s *retroScreen) build(m retroModel) {
	s.stopSlides()
	s.slides = newRetroSlideLayout()
	s.preferenceKey = m.PreferenceKey
	s.handles = map[string]*retroColumnHandle{}
	s.draggingWidth = false
	s.pendingWidth = false
	s.widthGeneration++
	s.layout = m.Layout
	s.columns = append([]retroColumn(nil), m.Columns...)
	s.columnsOpen = m.ColumnsOpen
	s.hasMessage = m.Message != ""
	s.hasError = m.Error != ""
	s.hasSetup = m.SetupTitle != ""
	s.setupCount = len(m.Setup)
	s.rows = map[string]*retroRowView{}
	s.order = nil
	s.separators = nil
	s.checks = map[string]*widget.Check{}
	s.setupValues = nil
	title := retroText("F1 LIVE TIMING", retroWhite, 23)
	s.state = retroText("LIVE SESSION", retroGreen, 18)
	s.state.Alignment = fyne.TextAlignTrailing
	top := container.NewBorder(nil, nil, nil, s.state, title)
	s.subtitle = retroText("", retroYellow, 16)
	layout := m.Layout
	button := widget.NewButton("COLUMNS", func() { s.bridge.act(desktopAction{Kind: "columns", Layout: layout}) })
	header := container.NewVBox(top, container.NewBorder(nil, nil, nil, button, s.subtitle), canvas.NewRectangle(retroMuted))
	s.body = container.New(s.slides)
	if m.ColumnsOpen {
		choices := []fyne.CanvasObject{}
		for _, choice := range m.Choices {
			key := choice.Key
			check := widget.NewCheck(choice.Label, nil)
			check.SetChecked(choice.Visible)
			check.OnChanged = func(on bool) { s.bridge.act(desktopAction{Kind: "field", Key: key, Visible: on, Layout: layout}) }
			s.checks[key] = check
			choices = append(choices, check)
		}
		choices = append(choices, widget.NewButton("RESET DEFAULTS", func() { s.bridge.act(desktopAction{Kind: "reset", Layout: layout}) }))
		s.body.Add(container.NewGridWithColumns(4, choices...))
	}
	s.message = nil
	s.headings = nil
	if s.hasMessage {
		s.message = retroText(m.Message, retroYellow, 14)
		s.body.Add(s.message)
	} else {
		heads := []fyne.CanvasObject{}
		for i := range m.Columns {
			heads = append(heads, s.columnHeading(i))
		}
		s.headings = container.NewStack(canvas.NewRectangle(retroNavy), container.New(retroRowLayout{columns: s.columns, header: true}, heads...))
		s.body.Add(s.headings)
	}
	footer := container.NewVBox()
	s.setupTitle = nil
	if s.hasSetup {
		line := canvas.NewRectangle(retroYellow)
		line.SetMinSize(fyne.NewSize(0, 2))
		footer.Add(line)
		s.setupTitle = retroText(m.SetupTitle, retroYellow, 15)
		footer.Add(s.setupTitle)
		fields := []fyne.CanvasObject{}
		for _, value := range m.Setup {
			text := retroText(value, retroWhite, 12)
			s.setupValues = append(s.setupValues, text)
			fields = append(fields, text)
		}
		detail := container.NewVScroll(container.NewGridWithColumns(3, fields...))
		detail.SetMinSize(fyne.NewSize(0, 175))
		footer.Add(detail)
	}
	s.errorLabel = nil
	if s.hasError {
		s.errorLabel = widget.NewLabel(m.Error)
		footer.Add(s.errorLabel)
	}
	s.status = retroText(m.Status, retroMuted, 11)
	footer.Add(s.status)
	s.root = container.NewStack(canvas.NewRectangle(retroBlack), container.NewPadded(container.NewBorder(header, footer, nil, nil, container.NewScroll(s.body))))
}
func (s *retroScreen) makeRow(id string) *retroRowView {
	row := &retroRowView{background: canvas.NewRectangle(retroBlack)}
	objects := make([]fyne.CanvasObject, len(s.columns))
	for i, col := range s.columns {
		cell := retroCellView{text: retroText("", retroWhite, 14)}
		cell.label = fittedRetroText(cell.text)
		cell.node = cell.label
		if col.Key == "team" || col.Key == "tyre" {
			cell.image = canvas.NewImageFromResource(nil)
			cell.image.FillMode = canvas.ImageFillContain
			cell.image.Hide()
			cell.node = container.NewStack(cell.label, cell.image)
		}
		if col.Key == "setup" {
			layout := s.layout
			cell.node = tapRetro(cell.label, func() { s.bridge.act(desktopAction{Kind: "setup", Row: id, Layout: layout}) })
		}
		row.cells = append(row.cells, cell)
		objects[i] = cell.node
	}
	row.node = container.NewStack(row.background, container.New(retroRowLayout{columns: s.columns}, objects...))
	s.slides.register(row.node)
	return row
}
func (s *retroScreen) updateRow(view *retroRowView, row retroRow, used map[string]bool) {
	background := retroBlack
	if row.AtRisk {
		background = retroEliminationRed
	}
	if row.Player {
		background = retroBlue
	}
	if view.background.FillColor != background {
		view.background.FillColor = background
		view.background.Refresh()
	}
	for i, value := range row.Cells {
		cell := &view.cells[i]
		key := s.columns[i].Key
		text := value.Text
		if key == "driver" || key == "status" {
			text = strings.ToUpper(text)
		}
		if key == "position" && row.Player {
			text = "▸" + text
		}
		cell.label.set(text, retroCellColour(key, value, row.Retired))
		if cell.image == nil {
			continue
		}
		if value.PNG != "" {
			used[value.PNG] = true
		}
		if cell.png != value.PNG {
			cell.png = value.PNG
			res := s.images[value.PNG]
			if res == nil && value.PNG != "" {
				data := retroImageBytes(value.PNG)
				if _, err := png.DecodeConfig(bytes.NewReader(data)); err == nil {
					digest := sha256.Sum256(data)
					res = fyne.NewStaticResource(fmt.Sprintf("live-%x.png", digest[:16]), data)
					s.images[value.PNG] = res
				}
			}
			cell.image.Resource = res
			if res != nil {
				cell.text.Hide()
				cell.image.Show()
			} else {
				cell.image.Hide()
				cell.text.Show()
			}
			cell.image.Refresh()
		}
		opacity := float64(0)
		if row.Retired {
			opacity = 0.55
		}
		if cell.image.Translucency != opacity {
			cell.image.Translucency = opacity
			cell.image.Refresh()
		}
	}
}
func (s *retroScreen) content(m retroModel) fyne.CanvasObject {
	if s.images == nil {
		s.images = map[string]fyne.Resource{}
	}
	if s.root == nil || s.layout != m.Layout || !sameRetroColumns(s.columns, m.Columns) || s.preferenceKey != m.PreferenceKey || s.columnsOpen != m.ColumnsOpen || s.hasMessage != (m.Message != "") || s.hasError != (m.Error != "") || s.hasSetup != (m.SetupTitle != "") || s.setupCount != len(m.Setup) {
		s.build(m)
	}
	if m.DisableSlides && s.slides.enabled {
		s.stopSlides()
	}
	s.slides.enabled = !m.DisableSlides
	if !s.draggingWidth {
		if s.pendingWidth {
			for _, col := range m.Columns {
				if col.Key == s.pendingKey && (col.Width == s.pendingValue || m.Error != "") {
					s.pendingWidth = false
				}
			}
		}
		if !s.pendingWidth {
			changed := false
			for i, col := range m.Columns {
				if s.columns[i].Width != col.Width {
					s.columns[i].Width = col.Width
					changed = true
				}
			}
			if changed {
				s.body.Refresh()
			}
		}
	}
	state, colour := "LIVE SESSION", color.Color(retroGreen)
	if strings.HasPrefix(m.Status, "Paused") {
		state = "PAUSED"
		colour = retroYellow
	}
	setRetroText(s.state, state, colour)
	heading := strings.ToUpper(m.Title)
	if heading == "" {
		heading = "WAITING FOR SESSION"
	}
	setRetroText(s.subtitle, heading, retroYellow)
	setRetroText(s.status, m.Status, retroMuted)
	if s.message != nil {
		setRetroText(s.message, m.Message, retroYellow)
	}
	if s.errorLabel != nil && s.errorLabel.Text != m.Error {
		s.errorLabel.SetText(m.Error)
	}
	if s.setupTitle != nil {
		setRetroText(s.setupTitle, m.SetupTitle, retroYellow)
		for i, value := range m.Setup {
			setRetroText(s.setupValues[i], value, retroWhite)
		}
	}
	for _, choice := range m.Choices {
		if check := s.checks[choice.Key]; check != nil && check.Checked != choice.Visible {
			callback := check.OnChanged
			check.OnChanged = nil
			check.SetChecked(choice.Visible)
			check.OnChanged = callback
		}
	}
	used := map[string]bool{}
	if !s.hasMessage {
		reordered := len(s.order) != len(m.Rows)
		for i, row := range m.Rows {
			view := s.rows[row.ID]
			if view == nil {
				view = s.makeRow(row.ID)
				s.rows[row.ID] = view
			}
			s.updateRow(view, row, used)
			if i >= len(s.order) || s.order[i] != row.ID {
				reordered = true
			}
		}
		if reordered {
			// Keep column controls and headings, replacing only the ordered list of
			// existing row nodes. The scroll container and its offset stay intact.
			prefix := 1
			if s.columnsOpen {
				prefix++
			}
			objects := s.body.Objects[:prefix]
			s.order = s.order[:0]
			separator := 0
			for i, row := range m.Rows {
				objects = append(objects, s.rows[row.ID].node)
				s.order = append(s.order, row.ID)
				if (i+1)%5 == 0 && i+1 < len(m.Rows) {
					if separator == len(s.separators) {
						line := canvas.NewRectangle(retroYellow)
						line.SetMinSize(fyne.NewSize(0, 1))
						s.separators = append(s.separators, line)
					}
					objects = append(objects, s.separators[separator])
					separator++
				}
			}
			// Do not leave references to removed rows in an oversized backing array.
			for i := len(objects); i < len(s.body.Objects); i++ {
				s.body.Objects[i] = nil
			}
			s.body.Objects = objects
			s.slides.pending = len(s.order) > 0 && s.slides.enabled
			s.body.Refresh()
			for id := range s.rows {
				found := false
				for _, row := range m.Rows {
					if row.ID == id {
						found = true
						break
					}
				}
				if !found {
					delete(s.slides.slots, s.rows[id].node)
					delete(s.rows, id)
				}
			}
		}
	}
	for key := range s.images {
		if !used[key] {
			delete(s.images, key)
		}
	}
	return s.root
}

func (s *retroScreen) stopSlides() {
	if s.slides != nil {
		s.slides.stop(true)
	}
}

// Only main owns the desktop event loop. Protocol processing stays alive when
// the window is hidden, and EOF from Swofty quits both without a detached child.
func main() {
	bridge := newDesktopBridge()
	done := make(chan error, 1)
	go func() { done <- runWithDesktop(os.Stdin, os.Stdout, bridge) }()
	var first retroModel
	for {
		select {
		case err := <-done:
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			return
		case model := <-bridge.models:
			if model.Show {
				first = model
				goto open
			}
		}
	}
open:
	application := app.NewWithID(addonID + ".retro")
	icon := fyne.NewStaticResource("icon.png", retroIconPNG)
	application.SetIcon(icon)
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	window := application.NewWindow("F1 Live Telemetry · Retro")
	window.SetIcon(icon)
	screen := &retroScreen{bridge: bridge, images: map[string]fyne.Resource{}}
	window.Resize(fyne.NewSize(1600, 920))
	window.SetContent(screen.content(first))
	geometry := &windowGeometry{window: window, path: windowPreferencePath(first.DataDir)}
	window.SetCloseIntercept(func() {
		geometry.capture(true)
		screen.stopSlides()
		window.Hide()
		bridge.act(desktopAction{Kind: "dismiss"})
	})
	go func() {
		previous := first
		ticker := time.NewTicker(geometryPollInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fyne.Do(func() { geometry.capture(false) })
			case err := <-done:
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				fyne.Do(func() { geometry.capture(true); screen.stopSlides(); application.Quit() })
				return
			case model := <-bridge.models:
				if reflect.DeepEqual(previous, model) {
					continue
				}
				wasShown := previous.Show
				previous = model
				fyne.Do(func() {
					if model.Show {
						content := screen.content(model)
						if content != window.Content() {
							window.SetContent(content)
						}
						if !wasShown {
							window.Show()
							geometry.open()
						}
					} else if wasShown {
						geometry.capture(true)
						screen.stopSlides()
						window.Hide()
					}
				})
			}
		}
	}()
	finishPlacement := geometry.prepare()
	window.Show()
	finishPlacement()
	geometry.open()
	application.Run()
}
