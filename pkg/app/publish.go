package app

import (
	"bytes"
	"log/slog"
	"os"
	"regexp"
	"text/template"
	"time"

	"github.com/georgemblack/blue-report/pkg/app/util"
	"github.com/georgemblack/blue-report/pkg/storage"
	minify "github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

// Publish converts a report to HTML and JSON, and publishes to an S3 bucket where the site is hosted.
func Publish(report Report) error {
	slog.Info("starting report publish")
	start := time.Now()

	// Build storage client
	stg, err := storage.New()
	if err != nil {
		return util.WrapErr("failed to create storage client", err)
	}

	// Generate webpage and publish
	report.Archive = false
	result, err := convert(report)
	if err != nil {
		return util.WrapErr("failed to generate html", err)
	}

	if os.Getenv("DEBUG") == "true" {
		os.WriteFile("index.html", result, 0644)
	}

	err = stg.PublishSite(result)
	if err != nil {
		return util.WrapErr("failed to publish site", err)
	}

	// Generate a second copy of the webpage, and publish to the archive.
	// This version of the page has a disclosure in the header.
	report.Archive = true
	result, err = convert(report)
	if err != nil {
		return util.WrapErr("failed to generate html", err)
	}

	if os.Getenv("DEBUG") == "true" {
		os.WriteFile("archive.html", result, 0644)
	}

	err = stg.PublishArchive(result)
	if err != nil {
		return util.WrapErr("failed to publish archive", err)
	}

	duration := time.Since(start)
	slog.Info("publish complete", "seconds", duration.Seconds())
	return nil
}

func convert(report Report) ([]byte, error) {
	tmpl, err := template.ParseFS(indexTmpl, "assets/index.html")
	if err != nil {
		return nil, util.WrapErr("failed to parse template", err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, report)
	if err != nil {
		return nil, util.WrapErr("failed to execute template", err)
	}

	// Minify HTML
	minifier := minify.New()
	minifier.Add("text/html", &html.Minifier{
		KeepDefaultAttrVals: true,
		KeepDocumentTags:    true,
		KeepEndTags:         true,
		KeepQuotes:          true,
	})
	minifier.AddFunc("text/css", css.Minify)
	minifier.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)

	if os.Getenv("DEBUG") == "true" {
		return buf.Bytes(), nil
	}
	return minifier.Bytes("text/html", buf.Bytes())
}
