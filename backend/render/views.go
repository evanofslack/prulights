package render

import (
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/evanofslack/prulights/schedule"
)

// margin is the paper's edge.
const margin = 24

// The type scale. Everything is large: the frame is read at arm's length
// by someone who may not have their glasses on.
var (
	labelStyle   = style{w: bold, size: 20, ink: Black, spacing: 1.5}
	colorsStyle  = style{w: bold, size: 36, ink: Black}
	causeStyle   = style{w: regular, size: 28, ink: Black}
	bodyStyle    = style{w: regular, size: 21, ink: Black}
	bodyBold     = style{w: bold, size: 21, ink: Black}
	smallStyle   = style{w: regular, size: 18, ink: Black}
	headStyle    = style{w: bold, size: 32, ink: Black}
	bigNumber    = style{w: bold, size: 72, ink: Black}
	nightHead    = style{w: bold, size: 44, ink: White}
	nightBody    = style{w: regular, size: 24, ink: White}
	nightBodyDim = style{w: regular, size: 20, ink: White}
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
	c := newCanvas(Width, Height, White)
	switch v {
	case ViewTonight:
		tonightView(c, &d)
	case ViewWeek:
		weekView(c, &d)
	case ViewMonth:
		monthView(c, &d)
	case ViewLog:
		logView(c, &d)
	case ViewSetup:
		setupView(c, &d)
	case ViewGoodnight:
		goodnightView(c, &d)
	case ViewWifi:
		wifiView(c, &d)
	default:
		return nil, fmt.Errorf("render: unknown view %q", v)
	}
	overlays(c, &d, v)
	return &Frame{View: v, Pix: c.pix}, nil
}

// tonightView: the tower in tonight's colors over what they're for, the next
// three nights, and a QR code to rate tonight.
func tonightView(c *canvas, d *Data) {
	l := d.night(d.Tonight)
	drawTower(c, image.Rect(0, 0, Width, 430), l.Colors, true)

	w := Width - 2*margin
	y := 430 + 44
	c.text(margin, y, labelStyle, strings.ToUpper("Tonight · "+dayLabel(d.Tonight)))
	y += 46
	for _, line := range wrap(colorsStyle, colorNames(l.Colors), w, 2) {
		c.text(margin, y, colorsStyle, line)
		y += colorsStyle.lineHeight()
	}
	y -= colorsStyle.lineHeight() - 38
	lines := wrap(causeStyle, causeText(l), w, 2)
	for _, line := range lines {
		c.text(margin, y, causeStyle, line)
		y += causeStyle.lineHeight()
	}

	// the next three nights, with the QR code beside them, sit at the bottom
	const qrSize = 120
	top := Height - 16 - 3*38 - 8

	if s := d.sighting(d.Tonight); s.Seen || s.Rating > 0 {
		y += 4
		c.check(image.Pt(margin, y-24), 26, Black)
		x := c.text(margin+34, y, bodyBold, "Seen")
		if s.Rating > 0 {
			stars(c, x+20, y-9, 12, s.Rating)
		}
		y += 36
	}
	if y+6 <= top-20 && !l.LightsOn.IsZero() {
		c.text(margin, y+4, bodyStyle, lightsOnText(l))
		y += 36
	}
	if y+6 > top {
		top = y + 6
	}
	c.fill(image.Rect(margin, top-14, Width-margin, top-12), Solid(Black))
	rowW := Width - 2*margin - qrSize - 12
	for i := 1; i <= 3; i++ {
		n := d.night(d.Tonight.AddDays(i))
		ry := top + (i-1)*38
		swatch(c, image.Rect(margin, ry+6, margin+40, ry+30), n.Colors)
		x := c.text(margin+52, ry+26, bodyBold, n.Date.Time().Format("Mon"))
		c.text(x+10, ry+26, bodyStyle, ellipsize(bodyStyle, shortCause(n), margin+rowW-x-10))
	}
	qr(c, image.Pt(Width-margin-qrSize+6, top-6), qrSize, d.nightURL(d.Tonight))
}

// lightsOnText is "Lights on at 5 pm" (the source says so) or "Lights on
// around 4:12 pm" (sunset).
func lightsOnText(l schedule.Lighting) string {
	t := l.LightsOn.In(schedule.Boston)
	clock := t.Format("3:04 pm")
	if t.Minute() == 0 {
		clock = t.Format("3 pm")
	}
	if l.LightsOnSource == schedule.LightsOnSource {
		return "Lights on at " + clock
	}
	return "Lights on around " + clock
}

// stars draws five stars centered on cy starting at x, the first rating
// filled, and returns where they end.
func stars(c *canvas, x, cy int, r float64, rating int) int {
	for i := range 5 {
		cx := float64(x) + r + float64(i)*(2*r+6)
		c.star(pt{cx, float64(cy)}, r, Black, i >= rating)
	}
	return x + int(5*(2*r+6))
}

// weekView: seven nights from tonight, a row each.
func weekView(c *canvas, d *Data) {
	end := d.Tonight.AddDays(6)
	c.text(margin, 48, labelStyle, "THE WEEK AHEAD")
	c.text(margin, 88, headStyle, d.Tonight.Time().Format("Jan 2")+" – "+end.Time().Format("Jan 2"))
	const top, rowH = 112, 96
	textX := margin + 104
	textW := Width - margin - textX
	for i := range 7 {
		day := d.Tonight.AddDays(i)
		n := d.night(day)
		y := top + i*rowH
		c.fill(image.Rect(margin, y, Width-margin, y+2), Solid(Black))
		swatch(c, image.Rect(margin, y+14, margin+88, y+rowH-14), n.Colors)
		head := dayLabel(day)
		if i == 0 {
			head = "Tonight, " + head
		}
		x := c.text(textX, y+32, bodyBold, head)
		if s := d.sighting(day); s.Seen {
			c.check(image.Pt(x+10, y+12), 22, Black)
		}
		c.text(textX, y+58, bodyStyle, ellipsize(bodyStyle, colorNames(n.Colors), textW))
		c.text(textX, y+84, smallStyle, ellipsize(smallStyle, shortCause(n), textW))
	}
}

// monthView: tonight's month as a calendar, each day filled with its colors.
func monthView(c *canvas, d *Data) {
	t := d.Tonight.Time()
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, schedule.Boston)
	days := first.AddDate(0, 1, -1).Day()
	lead := int(first.Weekday())
	rows := (lead + days + 6) / 7

	c.text(margin, 52, headStyle, strings.ToUpper(t.Format("January 2006")))
	const gx, top = 12, 106
	cellW := (Width - 2*gx) / 7
	cellH := (Height - 12 - top) / rows
	for i, wd := range []string{"S", "M", "T", "W", "T", "F", "S"} {
		c.textCenter(gx+i*cellW+cellW/2, 92, style{w: bold, size: 18, ink: Black}, wd)
	}
	numStyle := style{w: bold, size: 18, ink: Black}
	for day := 1; day <= days; day++ {
		pos := lead + day - 1
		col, row := pos%7, pos/7
		cell := image.Rect(gx+col*cellW, top+row*cellH, gx+(col+1)*cellW, top+(row+1)*cellH)
		date := schedule.DateOf(time.Date(t.Year(), t.Month(), day, 12, 0, 0, 0, schedule.Boston))
		n := d.night(date)
		inner := cell.Inset(3)
		swatch(c, image.Rect(inner.Min.X, inner.Min.Y+24, inner.Max.X, inner.Max.Y), n.Colors)
		label := fmt.Sprint(day)
		if date == d.Tonight {
			c.outline(cell, 4, Black)
			c.fill(image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, inner.Min.Y+24), Solid(Black))
			c.text(inner.Min.X+5, inner.Min.Y+19, style{w: bold, size: 18, ink: White}, label)
		} else {
			c.text(inner.Min.X+3, inner.Min.Y+19, numStyle, label)
		}
		if s := d.sighting(date); s.Seen {
			c.check(image.Pt(inner.Max.X-20, inner.Min.Y+3), 17, Black)
		}
	}
}

// logView: what she's seen and how she rated it.
func logView(c *canvas, d *Data) {
	c.text(margin, 48, labelStyle, "YOUR PRU LOG")
	st := d.Stats
	if st == nil || st.Seen == 0 {
		lines := wrap(causeStyle, "No nights logged yet. Hold the button when you see the crown lit to log it.", Width-2*margin, 4)
		for i, line := range lines {
			c.text(margin, 120+i*causeStyle.lineHeight(), causeStyle, line)
		}
		return
	}
	// two big numbers
	c.text(margin, 140, bigNumber, fmt.Sprint(st.Seen))
	c.text(margin, 176, bodyStyle, "nights seen")
	if st.Rated > 0 {
		x := Width/2 + 10
		end := c.text(x, 140, bigNumber, fmt.Sprintf("%.1f", st.AvgRating))
		c.star(pt{float64(end + 22), 112}, 18, Black, false)
		c.text(x, 176, bodyStyle, fmt.Sprintf("average of %d", st.Rated))
	}

	y := 232
	if len(st.Favorites) > 0 {
		c.text(margin, y, labelStyle, "FAVORITE COLORS")
		y += 14
		for i, f := range st.Favorites {
			if i == 4 {
				break
			}
			swatch(c, image.Rect(margin, y+8, margin+56, y+36), []schedule.ColorKey{f.Color})
			name := colorNames([]schedule.ColorKey{f.Color})
			c.text(margin+72, y+30, bodyBold, name)
			c.textRight(Width-margin, y+30, bodyStyle, fmt.Sprintf("%d nights", f.Count))
			y += 42
		}
		y += 40
	}
	if len(st.Recent) > 0 {
		c.text(margin, y, labelStyle, "RECENT RATINGS")
		y += 14
		for i, r := range st.Recent {
			if i == 4 || y+60 > Height {
				break
			}
			swatch(c, image.Rect(margin, y+10, margin+56, y+44), r.Colors)
			c.text(margin+72, y+30, bodyBold, dayLabel(r.Date))
			stars(c, Width-margin-5*(2*11+6), y+22, 11, r.Rating)
			c.text(margin+72, y+54, smallStyle, ellipsize(smallStyle, r.Cause, Width-2*margin-72))
			y += 66
		}
	}
}

// setupView: the QR code that links the frame to an account.
func setupView(c *canvas, d *Data) {
	l := d.night(d.Tonight)
	drawTower(c, image.Rect(0, 0, Width, 300), l.Colors, true)
	w := Width - 2*margin
	c.text(margin, 340, labelStyle, strings.ToUpper("Tonight · "+dayLabel(d.Tonight)))
	c.text(margin, 384, colorsStyle, fit(colorsStyle, colorNames(l.Colors), w))

	c.textCenter(Width/2, 432, style{w: bold, size: 26, ink: Black}, "Scan to connect your phone")
	const size = 250
	box := qr(c, image.Pt((Width-size)/2, 446), size, d.ClaimURL)
	if d.ClaimCode != "" {
		y := box.Max.Y + 34
		if host := hostOf(d.ClaimURL); host != "" {
			c.textCenter(Width/2, y, smallStyle, "or enter this code at "+host+"/claim")
		}
		c.textCenter(Width/2, y+44, style{w: bold, size: 36, ink: Black, spacing: 4}, d.ClaimCode)
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
func wifiView(c *canvas, d *Data) {
	w := Width - 2*margin
	c.text(margin, 48, labelStyle, "STEP 1 OF 2")
	y := 96
	for _, line := range wrap(headStyle, "Connect the frame to Wi-Fi", w, 2) {
		c.text(margin, y, headStyle, line)
		y += headStyle.lineHeight()
	}
	for _, line := range wrap(bodyStyle, "Scan this with your phone's camera to join the frame's own network:", w, 2) {
		c.text(margin, y+6, bodyStyle, line)
		y += bodyStyle.lineHeight()
	}
	const size = 250
	box := qr(c, image.Pt((Width-size)/2, y+18), size, wifiQR(d.APSSID, d.APPassword))
	y = box.Max.Y + 44
	c.text(margin, y, bodyStyle, "or join it by hand:")
	y += 40
	x := c.text(margin, y, bodyStyle, "Network ")
	c.text(x+6, y, bodyBold, d.APSSID)
	y += 34
	x = c.text(margin, y, bodyStyle, "Password ")
	c.text(x+6, y, bodyBold, d.APPassword)
	y += 34
	portal := d.PortalURL
	if portal == "" {
		portal = "http://192.168.4.1"
	}
	x = c.text(margin, y, bodyStyle, "then open ")
	c.text(x+6, y, bodyBold, strings.TrimPrefix(portal, "http://"))
	y += 48
	for _, line := range wrap(smallStyle, "Pick your Wi-Fi there. Next, the frame shows a code to connect your phone.", w, 2) {
		c.text(margin, y, smallStyle, line)
		y += smallStyle.lineHeight()
	}
}

// wifiQR is the text phone cameras read as "join this network".
func wifiQR(ssid, password string) string {
	esc := strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, `:`, `\:`, `"`, `\"`)
	return "WIFI:T:WPA;S:" + esc.Replace(ssid) + ";P:" + esc.Replace(password) + ";;"
}

// goodnightView: quiet hours, a small tower and tonight's colors on black.
func goodnightView(c *canvas, d *Data) {
	c.fill(image.Rect(0, 0, Width, Height), Solid(Black))
	l := d.night(d.Tonight)
	drawTower(c, image.Rect(90, 40, 390, 480), l.Colors, true)
	c.fill(image.Rect(0, 480, Width, Height), Solid(Black))
	c.textCenter(Width/2, 560, nightHead, "Good night")
	y := 612
	for _, line := range wrap(nightBody, "Tonight was "+colorNames(l.Colors)+".", Width-2*margin, 2) {
		c.textCenter(Width/2, y, nightBody, line)
		y += nightBody.lineHeight()
	}
	if l.Scheduled && l.Cause != "" {
		for _, line := range wrap(nightBodyDim, "For "+l.Cause+".", Width-2*margin, 2) {
			c.textCenter(Width/2, y, nightBodyDim, line)
			y += nightBodyDim.lineHeight()
		}
	}
}

// overlays: the stale-schedule banner and the low-battery mark.
func overlays(c *canvas, d *Data, v View) {
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
