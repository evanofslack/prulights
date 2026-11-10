package render

import (
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// margin is the page's edge.
const margin = 28

// The type scale. Everything is large: the frame is read at arm's length
// by someone who may not have their glasses on. Styles carry black; views
// put them in the theme's ink with on().
var (
	labelStyle  = style{w: bold, size: 18, spacing: 2}
	colorsStyle = style{w: bold, size: 40}
	causeStyle  = style{w: regular, size: 25}
	bodyStyle   = style{w: regular, size: 21}
	bodyBold    = style{w: bold, size: 22}
	smallStyle  = style{w: regular, size: 18}
	headStyle   = style{w: bold, size: 32}
	bigNumber   = style{w: bold, size: 72}
	nightHead   = style{w: bold, size: 46}
)

// renderMu runs one render at a time. The cached font faces aren't safe to
// share: an opentype face reuses its buffers between calls. Renders take
// milliseconds and frames get cached by hash, so this costs nothing.
var renderMu sync.Mutex

// Render draws a view. It's safe to call from several goroutines.
func Render(v View, d Data) (*Frame, error) {
	if d.Tonight == "" {
		return nil, fmt.Errorf("render %s: no tonight date", v)
	}
	renderMu.Lock()
	defer renderMu.Unlock()
	th := themeOf(d.Look)
	c := newCanvas(Width, Height, th.bg)
	switch v {
	case ViewTonight:
		tonightView(c, &d, th)
	case ViewWeek:
		weekView(c, &d, th)
	case ViewMonth:
		monthView(c, &d, th)
	case ViewLog:
		logView(c, &d, th)
	case ViewSetup:
		setupView(c, &d, th)
	case ViewGoodnight:
		goodnightView(c, &d, th)
	case ViewWifi:
		wifiView(c, &d, th)
	default:
		return nil, fmt.Errorf("render: unknown view %q", v)
	}
	overlays(c, &d, v, th)
	return &Frame{View: v, Pix: c.pix}, nil
}

// tonightView: the tower, as large as the page allows, lit in tonight's
// colors. Under it, as the owner likes: nothing, the date and colors, or
// those and what they're for.
func tonightView(c *canvas, d *Data, th theme) {
	l := d.night(d.Tonight)
	words := d.Look.words()
	w := Width - 2*margin
	label, colorsSt, causeSt := labelStyle.on(th), colorsStyle.on(th), causeStyle.on(th)

	var names, cause []string
	h := 0
	if words != WordsNone {
		names = wrap(colorsSt, colorNames(l.Colors), w, 2)
		h = 30 + 14 + len(names)*colorsSt.lineHeight()
		if words == WordsCause {
			cause = wrap(causeSt, causeText(l), w, 2)
			h += len(cause) * causeSt.lineHeight()
		}
		h += 18
	}
	bottom := Height - h
	drawScene(c, newScene(d.Look, th, l.Colors, bottom))
	if words == WordsNone {
		return
	}

	y := bottom + 30
	c.text(margin, y, label, strings.ToUpper("Tonight · "+dayLabel(d.Tonight)))
	if s := d.sighting(d.Tonight); s.Seen || s.Rating > 0 {
		x := Width - margin
		if s.Rating > 0 {
			x = Width - margin - 5*(2*9+6)
			stars(c, x, y-7, 9, s.Rating, th.fg)
		}
		c.check(image.Pt(x-30, y-20), 22, th.fg)
	}
	y += 14 + int(colorsSt.size*0.82)
	for _, line := range names {
		c.text(margin, y, colorsSt, line)
		y += colorsSt.lineHeight()
	}
	y -= colorsSt.lineHeight() - causeSt.lineHeight() - 4
	for _, line := range cause {
		c.text(margin, y, causeSt, line)
		y += causeSt.lineHeight()
	}
}

// stars draws five stars centered on cy starting at x, the first rating
// filled, and returns where they end.
func stars(c *canvas, x, cy int, r float64, rating int, ink Ink) int {
	for i := range 5 {
		cx := float64(x) + r + float64(i)*(2*r+6)
		c.star(pt{cx, float64(cy)}, r, ink, i >= rating)
	}
	return x + int(5*(2*r+6))
}

// weekView: seven nights from tonight, each a small lit crown and its words.
func weekView(c *canvas, d *Data, th theme) {
	end := d.Tonight.AddDays(6)
	c.text(margin, 48, labelStyle.on(th), "THE WEEK AHEAD")
	span := d.Tonight.Time().Format("Jan 2") + " – " + end.Time().Format("Jan 2")
	if d.Tonight.Time().Month() == end.Time().Month() {
		span = d.Tonight.Time().Format("Jan 2") + " – " + end.Time().Format("2")
	}
	c.text(margin, 90, headStyle.on(th), span)
	const top, rowH = 112, 96
	textX := margin + 92
	textW := Width - margin - textX
	head, body, small := bodyBold.on(th), bodyStyle.on(th), smallStyle.on(th)
	for i := range 7 {
		day := d.Tonight.AddDays(i)
		n := d.night(day)
		y := top + i*rowH
		c.fill(image.Rect(margin, y, Width-margin, y+1), th.rule)
		crownIcon(c, image.Rect(margin+4, y+12, margin+68, y+rowH-10), n.Colors, th)
		name := dayLabel(day)
		if i == 0 {
			name = "Tonight"
		}
		x := c.text(textX, y+32, head, name)
		if s := d.sighting(day); s.Seen {
			c.check(image.Pt(x+10, y+12), 22, th.fg)
		}
		c.text(textX, y+59, body, ellipsize(body, colorNames(n.Colors), textW))
		if n.Scheduled && n.Cause != "" {
			c.text(textX, y+84, small, ellipsize(small, n.Cause, textW))
		}
	}
}

// monthView: tonight's month as a calendar, a small crown on each night.
func monthView(c *canvas, d *Data, th theme) {
	t := d.Tonight.Time()
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, schedule.Boston)
	days := first.AddDate(0, 1, -1).Day()
	lead := int(first.Weekday())
	rows := (lead + days + 6) / 7

	c.text(margin, 52, style{w: bold, size: 30, spacing: 2}.on(th), strings.ToUpper(t.Format("January 2006")))
	const gx, top = 12, 104
	cellW := (Width - 2*gx) / 7
	cellH := (Height - 10 - top) / rows
	wd := style{w: bold, size: 16, spacing: 1}.on(th)
	for i, name := range []string{"S", "M", "T", "W", "T", "F", "S"} {
		c.textCenter(gx+i*cellW+cellW/2, 90, wd, name)
	}
	num := style{w: bold, size: 18}.on(th)
	for day := 1; day <= days; day++ {
		pos := lead + day - 1
		col, row := pos%7, pos/7
		cell := image.Rect(gx+col*cellW, top+row*cellH, gx+(col+1)*cellW, top+(row+1)*cellH)
		date := schedule.DateOf(time.Date(t.Year(), t.Month(), day, 12, 0, 0, 0, schedule.Boston))
		n := d.night(date)
		iw := cellW - 16
		ih := cellH - 40
		icx := (cell.Min.X + cell.Max.X) / 2
		crownIcon(c, image.Rect(icx-iw/2, cell.Min.Y+30, icx+iw/2, cell.Min.Y+30+ih), n.Colors, th)
		label := fmt.Sprint(day)
		if date == d.Tonight {
			c.outline(cell.Inset(1), 3, th.fg)
			c.fill(image.Rect(cell.Min.X+1, cell.Min.Y+1, cell.Min.X+8+num.width(label)+6, cell.Min.Y+26), Solid(th.fg))
			c.text(cell.Min.X+7, cell.Min.Y+21, style{w: bold, size: 18, ink: th.bg}, label)
		} else {
			c.text(cell.Min.X+6, cell.Min.Y+21, num, label)
		}
		if s := d.sighting(date); s.Seen {
			c.check(image.Pt(cell.Max.X-22, cell.Min.Y+5), 16, th.fg)
		}
	}
}

// logView: how many nights she's seen, her average, her colors, and the
// nights themselves as a row of small crowns; a code to rate tonight.
func logView(c *canvas, d *Data, th theme) {
	label, body, small := labelStyle.on(th), bodyStyle.on(th), smallStyle.on(th)
	c.text(margin, 48, label, "YOUR PRU LOG")
	st := d.Stats
	if st == nil || st.Seen == 0 {
		lines := wrap(causeStyle.on(th), "No nights logged yet. Hold the button when you see the crown lit to log it.", Width-2*margin, 4)
		for i, line := range lines {
			c.text(margin, 120+i*causeStyle.lineHeight(), causeStyle.on(th), line)
		}
		rateTonight(c, d, th)
		return
	}
	c.text(margin, 140, bigNumber.on(th), fmt.Sprint(st.Seen))
	c.text(margin, 176, body, "nights seen")
	if st.Rated > 0 {
		x := Width/2 + 10
		end := c.text(x, 140, bigNumber.on(th), fmt.Sprintf("%.1f", st.AvgRating))
		c.star(pt{float64(end + 22), 112}, 18, th.fg, false)
		c.text(x, 176, body, fmt.Sprintf("average of %d", st.Rated))
	}

	y := 230
	if len(st.Favorites) > 0 {
		c.text(margin, y, label, "FAVORITE COLORS")
		y += 18
		n := min(4, len(st.Favorites))
		colW := (Width - 2*margin) / 4
		for i, f := range st.Favorites[:n] {
			x := margin + i*colW
			colorBar(c, image.Rect(x, y, x+colW-14, y+14), []schedule.ColorKey{f.Color}, th)
			c.text(x, y+40, bodyBold.on(th), ellipsize(bodyBold, colorNames([]schedule.ColorKey{f.Color}), colW-8))
			c.text(x, y+64, small, fmt.Sprintf("%d nights", f.Count))
		}
		y += 112
	}
	if len(st.Nights) > 0 {
		c.text(margin, y, label, "LATELY")
		y += 16
		colW := (Width - 2*margin) / 4
		for i, n := range st.Nights {
			if i == 8 {
				break
			}
			x := margin + (i%4)*colW
			ry := y + (i/4)*124
			crownIcon(c, image.Rect(x+12, ry+4, x+colW-22, ry+66), n.Colors, th)
			c.text(x+4, ry+90, small, n.Date.Time().Format("Jan 2"))
			if n.Rating > 0 {
				stars(c, x+2, ry+106, 6, n.Rating, th.fg)
			}
		}
	}
	rateTonight(c, d, th)
}

// rateTonight is a QR code to tonight's page, in the log's bottom corner.
func rateTonight(c *canvas, d *Data, th theme) {
	const size = 96
	at := image.Pt(Width-margin-size, Height-margin-size+8)
	box := qrCard(c, at, size, d.nightURL(d.Tonight), th)
	s := smallStyle.on(th)
	c.textRight(box.Min.X-16, box.Max.Y-34, s, "Scan to rate")
	c.textRight(box.Min.X-16, box.Max.Y-10, s, "tonight")
}

// qrCard is a QR code that phones can read on either theme: always black on
// white, with a white margin on the night.
func qrCard(c *canvas, at image.Point, size int, text string, th theme) image.Rectangle {
	if !th.light {
		c.fill(image.Rect(at.X-8, at.Y-8, at.X+size+8, at.Y+size+8), Solid(White))
	}
	return qr(c, at, size, text)
}

// setupView: the QR code that links the frame to an account, under the top
// of the tower in tonight's colors.
func setupView(c *canvas, d *Data, th theme) {
	l := d.night(d.Tonight)
	s := newScene(Look{Light: th.light, Scene: SceneTower}, th, l.Colors, 330)
	drawScene(c, s)
	c.fade(image.Rect(0, 230, Width, 330), th.bg, false)

	c.textCenter(Width/2, 372, style{w: bold, size: 28}.on(th), "Scan to connect your phone")
	const size = 240
	box := qrCard(c, image.Pt((Width-size)/2, 400), size, d.ClaimURL, th)
	if d.ClaimCode != "" {
		y := box.Max.Y + 44
		if host := hostOf(d.ClaimURL); host != "" {
			c.textCenter(Width/2, y, smallStyle.on(th), "or enter this code at "+host+"/claim")
		}
		c.textCenter(Width/2, y+48, style{w: bold, size: 38, spacing: 4}.on(th), d.ClaimCode)
	}
}

// hostOf is a url's host, for telling someone where to type a code.
func hostOf(u string) string {
	rest, ok := strings.CutPrefix(u, "https://")
	if !ok {
		rest, _ = strings.CutPrefix(u, "http://")
	}
	host, _, _ := strings.Cut(rest, "/")
	return host
}

// wifiView: the frame can't reach any network it knows. Join its own
// hotspot (scan, or type) and pick a network on its page. It's step 1; the
// welcome screen's code is step 2.
func wifiView(c *canvas, d *Data, th theme) {
	w := Width - 2*margin
	head, body, bodyB, small := headStyle.on(th), bodyStyle.on(th), bodyBold.on(th), smallStyle.on(th)
	c.text(margin, 48, labelStyle.on(th), "STEP 1 OF 2")
	y := 96
	for _, line := range wrap(head, "Connect the frame to Wi-Fi", w, 2) {
		c.text(margin, y, head, line)
		y += head.lineHeight()
	}
	for _, line := range wrap(body, "Scan this with your phone's camera to join the frame's own network:", w, 2) {
		c.text(margin, y+6, body, line)
		y += body.lineHeight()
	}
	const size = 240
	box := qrCard(c, image.Pt((Width-size)/2, y+24), size, wifiQR(d.APSSID, d.APPassword), th)
	y = box.Max.Y + 50
	c.text(margin, y, body, "or join it by hand:")
	y += 40
	x := c.text(margin, y, body, "Network ")
	c.text(x+6, y, bodyB, d.APSSID)
	y += 34
	x = c.text(margin, y, body, "Password ")
	c.text(x+6, y, bodyB, d.APPassword)
	y += 34
	portal := d.PortalURL
	if portal == "" {
		portal = "http://192.168.4.1"
	}
	x = c.text(margin, y, body, "then open ")
	c.text(x+6, y, bodyB, strings.TrimPrefix(portal, "http://"))
	y += 46
	for _, line := range wrap(small, "Pick your Wi-Fi there. Next, the frame shows a code to connect your phone.", w, 2) {
		c.text(margin, y, small, line)
		y += small.lineHeight()
	}
}

// wifiQR is the text phone cameras read as "join this network".
func wifiQR(ssid, password string) string {
	esc := strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, `:`, `\:`, `"`, `\"`)
	return "WIFI:T:WPA;S:" + esc.Replace(ssid) + ";P:" + esc.Replace(password) + ";;"
}

// goodnightView: quiet hours. Just the top of the tower, still lit, fading
// into the dark, and a line about tonight.
func goodnightView(c *canvas, d *Data, th theme) {
	l := d.night(d.Tonight)
	s := newScene(Look{Light: th.light, Scene: SceneTower}, th, l.Colors, 470)
	s.top = 96
	drawScene(c, s)
	c.fade(image.Rect(0, 330, Width, 470), th.bg, false)
	c.textCenter(Width/2, 560, nightHead.on(th), "Good night")
	body := style{w: regular, size: 24}.on(th)
	y := 612
	for _, line := range wrap(body, "Tonight was "+colorNames(l.Colors)+".", Width-2*margin, 2) {
		c.textCenter(Width/2, y, body, line)
		y += body.lineHeight()
	}
}

// overlays: the stale-schedule banner and the low-battery mark.
func overlays(c *canvas, d *Data, v View, th theme) {
	if d.StaleSince != "" && v != ViewSetup {
		c.fill(image.Rect(0, 0, Width, 34), Solid(Yellow))
		c.fill(image.Rect(0, 34, Width, 36), Solid(Black))
		c.textCenter(Width/2, 24, style{w: bold, size: 18, ink: Black},
			"Schedule as of "+d.StaleSince.Time().Format("Jan 2")+" · couldn't update")
	}
	if d.LowBattery {
		top := 8
		if d.StaleSince != "" && v != ViewSetup {
			top = 44
		}
		r := image.Rect(Width-margin-44, top, Width-margin, top+24)
		c.fill(r.Inset(-3), Solid(Black))
		c.outline(r, 2, White)
		c.fill(image.Rect(r.Max.X, r.Min.Y+7, r.Max.X+4, r.Max.Y-7), Solid(White))
		c.fill(image.Rect(r.Min.X+4, r.Min.Y+4, r.Min.X+11, r.Max.Y-4), Solid(Red))
	}
}
