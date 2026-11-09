package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/evanofslack/prulights/render"
	"github.com/evanofslack/prulights/schedule"
)

// renderCmd draws one of the frame's views to a png, from sample data.
func renderCmd(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	view := fs.String("view", string(render.ViewTonight), "view to draw: tonight, week, month, log, setup, goodnight")
	date := fs.String("date", string(schedule.Tonight(time.Now())), "tonight's date, YYYY-MM-DD")
	palette := fs.String("palette", "measured", "png colors: measured (how the panel looks) or ideal")
	out := fs.String("out", "", "png file to write (default render-<view>.png)")
	bin := fs.String("bin", "", "also write the gzipped panel framebuffer here")
	stale := fs.String("stale", "", "draw the stale-schedule banner, as of this date")
	lowBattery := fs.Bool("low-battery", false, "draw the low-battery mark")
	fs.Parse(args)

	d, err := schedule.ParseDate(*date)
	if err != nil {
		return err
	}
	data := render.SampleData(d)
	if *stale != "" {
		if data.StaleSince, err = schedule.ParseDate(*stale); err != nil {
			return err
		}
	}
	data.LowBattery = *lowBattery
	f, err := render.Render(render.View(*view), data)
	if err != nil {
		return err
	}
	if *out == "" {
		*out = "render-" + *view + ".png"
	}
	if err := os.WriteFile(*out, f.PNG(render.PaletteNamed(*palette)), 0o644); err != nil {
		return err
	}
	fmt.Println(*out)
	if *bin != "" {
		if err := os.WriteFile(*bin, f.PanelBin(), 0o644); err != nil {
			return err
		}
		fmt.Println(*bin, f.SHA256())
	}
	return nil
}
