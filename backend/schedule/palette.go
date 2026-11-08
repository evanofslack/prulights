package schedule

import (
	"regexp"
	"slices"
	"strings"
)

// ColorKey names one of the crown's colors.
type ColorKey string

const (
	White   ColorKey = "white"
	Red     ColorKey = "red"
	Blue    ColorKey = "blue"
	Green   ColorKey = "green"
	Yellow  ColorKey = "yellow"
	Gold    ColorKey = "gold"
	Orange  ColorKey = "orange"
	Pink    ColorKey = "pink"
	Purple  ColorKey = "purple"
	Teal    ColorKey = "teal"
	Maroon  ColorKey = "maroon"
	Black   ColorKey = "black"
	Silver  ColorKey = "silver"
	Rainbow ColorKey = "rainbow"
)

// Pigment is one of the six inks of an E Ink Spectra 6 panel.
type Pigment string

const (
	InkBlack  Pigment = "black"
	InkWhite  Pigment = "white"
	InkYellow Pigment = "yellow"
	InkRed    Pigment = "red"
	InkBlue   Pigment = "blue"
	InkGreen  Pigment = "green"
)

// InkRecipe is how a color is drawn on the six-color panel: pigment A, with
// Mix (0–1) of its pixels swapped for pigment B by an ordered dither.
type InkRecipe struct {
	A   Pigment `json:"a"`
	B   Pigment `json:"b,omitempty"`
	Mix float64 `json:"mix,omitempty"`
}

// Color is a palette entry: how to show a crown color on a screen and on
// e-ink. Rainbow has Bands instead of one hex.
type Color struct {
	Key   ColorKey   `json:"key"`
	Label string     `json:"label"`
	Hex   string     `json:"hex"`
	Ink   InkRecipe  `json:"ink"`
	Bands []ColorKey `json:"bands,omitempty"`
}

var palette = []Color{
	{Key: White, Label: "White", Hex: "#ffffff", Ink: InkRecipe{A: InkWhite}},
	{Key: Red, Label: "Red", Hex: "#e3242b", Ink: InkRecipe{A: InkRed}},
	{Key: Blue, Label: "Blue", Hex: "#1f63e0", Ink: InkRecipe{A: InkBlue}},
	{Key: Green, Label: "Green", Hex: "#1fa34f", Ink: InkRecipe{A: InkGreen}},
	{Key: Yellow, Label: "Yellow", Hex: "#ffd60a", Ink: InkRecipe{A: InkYellow}},
	{Key: Gold, Label: "Gold", Hex: "#f2b21b", Ink: InkRecipe{A: InkYellow, B: InkRed, Mix: 0.2}},
	{Key: Orange, Label: "Orange", Hex: "#ff7a1a", Ink: InkRecipe{A: InkYellow, B: InkRed, Mix: 0.5}},
	{Key: Pink, Label: "Pink", Hex: "#ff6fae", Ink: InkRecipe{A: InkWhite, B: InkRed, Mix: 0.45}},
	{Key: Purple, Label: "Purple", Hex: "#8b3ffc", Ink: InkRecipe{A: InkBlue, B: InkRed, Mix: 0.45}},
	{Key: Teal, Label: "Teal", Hex: "#14b8a6", Ink: InkRecipe{A: InkGreen, B: InkBlue, Mix: 0.45}},
	{Key: Maroon, Label: "Maroon", Hex: "#7d1f2e", Ink: InkRecipe{A: InkRed, B: InkBlack, Mix: 0.45}},
	{Key: Black, Label: "Black", Hex: "#14161c", Ink: InkRecipe{A: InkBlack}},
	{Key: Silver, Label: "Silver", Hex: "#c3c9d3", Ink: InkRecipe{A: InkWhite, B: InkBlack, Mix: 0.25}},
	{Key: Rainbow, Label: "Rainbow", Hex: "#ff7a1a", Ink: InkRecipe{A: InkYellow, B: InkRed, Mix: 0.5},
		Bands: []ColorKey{Red, Orange, Yellow, Green, Blue, Purple}},
}

// Palette returns every color, in a stable order.
func Palette() []Color { return slices.Clone(palette) }

// Lookup returns the palette entry for a key.
func Lookup(k ColorKey) (Color, bool) {
	for _, c := range palette {
		if c.Key == k {
			return c, true
		}
	}
	return Color{}, false
}

// aliases maps the words the source uses (lowercased) to palette keys. A
// value with several keys expands, like "patriotic".
var aliases = map[string][]ColorKey{
	"white":       {White},
	"red":         {Red},
	"crimson":     {Red},
	"scarlet":     {Red},
	"blue":        {Blue},
	"navy":        {Blue},
	"navy blue":   {Blue},
	"light blue":  {Blue},
	"royal blue":  {Blue},
	"green":       {Green},
	"kelly green": {Green},
	"emerald":     {Green},
	"yellow":      {Yellow},
	"gold":        {Gold},
	"golden":      {Gold},
	"amber":       {Gold},
	"orange":      {Orange},
	"pink":        {Pink},
	"magenta":     {Pink},
	"fuchsia":     {Pink},
	"purple":      {Purple},
	"violet":      {Purple},
	"lavender":    {Purple},
	"lilac":       {Purple},
	"teal":        {Teal},
	"turquoise":   {Teal},
	"aqua":        {Teal},
	"cyan":        {Teal},
	"maroon":      {Maroon},
	"burgundy":    {Maroon},
	"black":       {Black},
	"silver":      {Silver},
	"gray":        {Silver},
	"grey":        {Silver},
	"rainbow":     {Rainbow},
	"pride":       {Rainbow},
	"patriotic":   {Red, White, Blue},
}

var colorSplit = regexp.MustCompile(`\s*(?:,|&|\+|/|\band\b)\s*`)

// ParseColors turns the source's color phrase ("Blue, Pink, & Gold",
// "Red, White and Blue", "Patriotic") into palette keys, in order and without
// repeats. Words it doesn't know come back in unknown so the caller can warn;
// they're never silently dropped.
func ParseColors(raw string) (keys []ColorKey, unknown []string) {
	for _, part := range colorSplit.Split(strings.ToLower(raw), -1) {
		part = strings.Trim(part, " .;:-–— ")
		part = strings.TrimPrefix(part, "the ")
		if part == "" {
			continue
		}
		ks, ok := aliases[part]
		if !ok {
			unknown = append(unknown, part)
			continue
		}
		for _, k := range ks {
			if !slices.Contains(keys, k) {
				keys = append(keys, k)
			}
		}
	}
	return keys, unknown
}

// IsColorWord reports whether a word is a color the palette knows. The
// scraper uses it to find where the colors end and the cause begins.
func IsColorWord(w string) bool {
	_, ok := aliases[strings.ToLower(strings.TrimSpace(w))]
	return ok
}
