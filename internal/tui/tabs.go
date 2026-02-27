package tui

import "strings"

var tabNames = []string{"Provider", "Models", "Keys"}

func renderTabs(active int) string {
	var tabs []string
	for i, name := range tabNames {
		if i == active {
			tabs = append(tabs, styleActiveTab.Render(name))
		} else {
			tabs = append(tabs, styleInactiveTab.Render(name))
		}
	}
	return strings.Join(tabs, " ")
}
