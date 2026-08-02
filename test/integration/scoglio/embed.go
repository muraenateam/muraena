package main

import "embed"

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS
