package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/unidoc/unioffice/presentation"
)

// writePptxFile creates a .pptx file at dst. The content is split into slides
// by lines containing exactly "---"; each slide's first non-empty line becomes
// its title, remaining lines become bullets. An empty content produces a
// single blank slide.
func writePptxFile(ctx context.Context, dst, content, _ string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ppt := presentation.New()
	defer ppt.Close()

	slides := splitSlides(content)
	if len(slides) == 0 {
		slides = []slideContent{{Title: "", Body: nil}}
	}

	masters := ppt.SlideMasters()
	if len(masters) == 0 {
		return fmt.Errorf("no slide masters available")
	}
	masterLayouts := masters[0].SlideLayouts()
	if len(masterLayouts) == 0 {
		return fmt.Errorf("no slide layouts available")
	}
	defaultLayout := masterLayouts[0]

	for _, sl := range slides {
		s, err := ppt.AddDefaultSlideWithLayout(defaultLayout)
		if err != nil {
			return fmt.Errorf("add slide: %w", err)
		}
		if sl.Title != "" {
			// First placeholder = title
			placeholders := s.PlaceHolders()
			if len(placeholders) > 0 {
				placeholders[0].SetText(sl.Title)
			}
		}
		if len(sl.Body) > 0 && len(s.PlaceHolders()) > 1 {
			s.PlaceHolders()[1].SetText(strings.Join(sl.Body, "\n"))
		}
	}

	if err := ppt.SaveToFile(dst); err != nil {
		return fmt.Errorf("save pptx: %w", err)
	}
	return nil
}

type slideContent struct {
	Title string
	Body  []string
}

func splitSlides(content string) []slideContent {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	chunks := strings.Split(content, "\n---\n")
	out := make([]slideContent, 0, len(chunks))
	for _, chunk := range chunks {
		lines := strings.Split(chunk, "\n")
		var title string
		var body []string
		for i, l := range lines {
			if strings.TrimSpace(l) == "" {
				continue
			}
			if title == "" {
				title = l
				body = append(body, lines[i+1:]...)
				break
			}
		}
		out = append(out, slideContent{Title: title, Body: body})
	}
	return out
}
