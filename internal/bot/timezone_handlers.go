package bot

import (
	"fmt"
	"sort"
	"strings"
)

// tzByRegion holds a curated list of common IANA timezone names grouped by
// their region prefix. Users can browse regions then look up the exact name
// to paste into /schedule setup.
var tzByRegion = map[string][]string{
	"Africa": {
		"Africa/Abidjan", "Africa/Accra", "Africa/Addis_Ababa", "Africa/Algiers",
		"Africa/Cairo", "Africa/Casablanca", "Africa/Johannesburg", "Africa/Kampala",
		"Africa/Lagos", "Africa/Nairobi", "Africa/Tripoli", "Africa/Tunis",
	},
	"America": {
		"America/Anchorage", "America/Bogota", "America/Buenos_Aires",
		"America/Caracas", "America/Chicago", "America/Costa_Rica",
		"America/Denver", "America/Guayaquil", "America/Halifax",
		"America/Jamaica", "America/Lima", "America/Los_Angeles",
		"America/Managua", "America/Mexico_City", "America/Montevideo",
		"America/New_York", "America/Panama", "America/Phoenix",
		"America/Puerto_Rico", "America/Santiago", "America/Sao_Paulo",
		"America/St_Johns", "America/Toronto", "America/Vancouver",
	},
	"Asia": {
		"Asia/Almaty", "Asia/Amman", "Asia/Baghdad", "Asia/Baku",
		"Asia/Bangkok", "Asia/Beirut", "Asia/Colombo", "Asia/Dhaka",
		"Asia/Dubai", "Asia/Hong_Kong", "Asia/Irkutsk", "Asia/Jakarta",
		"Asia/Kabul", "Asia/Karachi", "Asia/Kathmandu", "Asia/Kolkata",
		"Asia/Krasnoyarsk", "Asia/Kuala_Lumpur", "Asia/Kuwait",
		"Asia/Magadan", "Asia/Manila", "Asia/Muscat", "Asia/Nicosia",
		"Asia/Novosibirsk", "Asia/Omsk", "Asia/Riyadh", "Asia/Seoul",
		"Asia/Shanghai", "Asia/Singapore", "Asia/Taipei", "Asia/Tashkent",
		"Asia/Tehran", "Asia/Tokyo", "Asia/Ulaanbaatar", "Asia/Vladivostok",
		"Asia/Yakutsk", "Asia/Yangon", "Asia/Yekaterinburg", "Asia/Yerevan",
	},
	"Atlantic": {
		"Atlantic/Azores", "Atlantic/Cape_Verde", "Atlantic/Reykjavik",
		"Atlantic/South_Georgia", "Atlantic/Stanley",
	},
	"Australia": {
		"Australia/Adelaide", "Australia/Brisbane", "Australia/Darwin",
		"Australia/Hobart", "Australia/Lord_Howe", "Australia/Melbourne",
		"Australia/Perth", "Australia/Sydney",
	},
	"Europe": {
		"Europe/Amsterdam", "Europe/Athens", "Europe/Belgrade", "Europe/Berlin",
		"Europe/Brussels", "Europe/Bucharest", "Europe/Budapest",
		"Europe/Copenhagen", "Europe/Dublin", "Europe/Helsinki",
		"Europe/Istanbul", "Europe/Kiev", "Europe/Lisbon", "Europe/Ljubljana",
		"Europe/London", "Europe/Luxembourg", "Europe/Madrid", "Europe/Minsk",
		"Europe/Moscow", "Europe/Oslo", "Europe/Paris", "Europe/Prague",
		"Europe/Riga", "Europe/Rome", "Europe/Sofia", "Europe/Stockholm",
		"Europe/Tallinn", "Europe/Vienna", "Europe/Vilnius", "Europe/Warsaw",
		"Europe/Zurich",
	},
	"Pacific": {
		"Pacific/Auckland", "Pacific/Chatham", "Pacific/Fiji",
		"Pacific/Guam", "Pacific/Honolulu", "Pacific/Marquesas",
		"Pacific/Midway", "Pacific/Norfolk", "Pacific/Noumea",
		"Pacific/Pago_Pago", "Pacific/Port_Moresby", "Pacific/Tongatapu",
	},
	"Etc": {
		"Etc/UTC",
		"Etc/GMT", "Etc/GMT+1", "Etc/GMT+2", "Etc/GMT+3", "Etc/GMT+4",
		"Etc/GMT+5", "Etc/GMT+6", "Etc/GMT+7", "Etc/GMT+8", "Etc/GMT+9",
		"Etc/GMT+10", "Etc/GMT+11", "Etc/GMT+12",
		"Etc/GMT-1", "Etc/GMT-2", "Etc/GMT-3", "Etc/GMT-4",
		"Etc/GMT-5", "Etc/GMT-6", "Etc/GMT-7", "Etc/GMT-8", "Etc/GMT-9",
		"Etc/GMT-10", "Etc/GMT-11", "Etc/GMT-12", "Etc/GMT-13", "Etc/GMT-14",
	},
}

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
