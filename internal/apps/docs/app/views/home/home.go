package home

import "docs/app"

var View = app.View[struct{}, struct{}]{
	Pattern:    "/",
	ClientFile: "./app/views/home/home.tsx",
}
