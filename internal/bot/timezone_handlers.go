package bot

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
	"strings"
)

//go:linkname zipdata time/tzdata.zipdata
var zipdata string

var tzByRegion = func() map[string][]string {
	r, err := zip.NewReader(bytes.NewReader([]byte(zipdata)), int64(len(zipdata)))
	if err != nil {
		return map[string][]string{}
	}
	m := make(map[string][]string)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		slash := strings.Index(name, "/")
		if slash < 0 {
			continue
		}
		region := name[:slash]
		m[region] = append(m[region], name)
	}
	for _, zones := range m {
		sort.Strings(zones)
	}
	return m
}()

func handleTimezone(args []string, notify func(string)) {
	if len(args) == 0 || args[0] != "list" {
		notify("Usage: /timezone list\n       /timezone list <region>\n\nExample: /timezone list Asia")
		return
	}

	// /timezone list — show all regions with counts
	if len(args) == 1 {
		regions := make([]string, 0, len(tzByRegion))
		for r := range tzByRegion {
			regions = append(regions, r)
		}
		sort.Strings(regions)

		var sb strings.Builder
		sb.WriteString("Timezone regions:\n")
		for _, r := range regions {
			sb.WriteString(fmt.Sprintf("\n  %-12s (%d entries)", r, len(tzByRegion[r])))
		}
		sb.WriteString("\n\nUse /timezone list <region> to see entries.\nExample: /timezone list Asia")
		notify(sb.String())
		return
	}

	// /timezone list <region> — show timezones in that region
	region := args[1]
	var matched string
	for r := range tzByRegion {
		if strings.EqualFold(r, region) {
			matched = r
			break
		}
	}
	if matched == "" {
		regions := make([]string, 0, len(tzByRegion))
		for r := range tzByRegion {
			regions = append(regions, r)
		}
		sort.Strings(regions)
		notify(fmt.Sprintf("Unknown region %q.\n\nValid regions: %s", region, strings.Join(regions, ", ")))
		return
	}

	zones := tzByRegion[matched]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s timezones:\n", matched))
	for _, z := range zones {
		sb.WriteString(fmt.Sprintf("\n  %s", z))
	}
	notify(sb.String())
}
