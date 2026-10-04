package aws

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// ListOptions controls filtering and presentation for ECS list commands.
type ListOptions struct {
	Output        string
	Sort          string
	Limit         int
	Name          string
	Status        string
	LaunchType    string
	DesiredStatus string
	Family        string
}

// ListColumn describes a field displayed by a list command.
type ListColumn struct {
	Key   string
	Title string
}

func RenderList(rows []map[string]any, columns []ListColumn, options ListOptions) error {
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if options.Name != "" && !strings.HasPrefix(strings.ToLower(fmt.Sprint(row["name"])), strings.ToLower(options.Name)) {
			continue
		}
		if options.Status != "" && !strings.EqualFold(fmt.Sprint(row["status"]), options.Status) {
			continue
		}
		if options.LaunchType != "" && !strings.EqualFold(fmt.Sprint(row["launchType"]), options.LaunchType) {
			continue
		}
		if options.DesiredStatus != "" && !strings.EqualFold(fmt.Sprint(row["desiredStatus"]), options.DesiredStatus) {
			continue
		}
		if options.Family != "" && !strings.HasPrefix(strings.ToLower(fmt.Sprint(row["family"])), strings.ToLower(options.Family)) {
			continue
		}
		filtered = append(filtered, row)
	}

	if options.Sort != "" {
		valid := false
		for _, column := range columns {
			if strings.EqualFold(column.Key, options.Sort) {
				options.Sort = column.Key
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("unsupported sort field %q (choose one of: %s)", options.Sort, listColumnKeys(columns))
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			return compareListValues(filtered[i][options.Sort], filtered[j][options.Sort]) < 0
		})
	}
	if options.Limit > 0 && len(filtered) > options.Limit {
		filtered = filtered[:options.Limit]
	}
	if filtered == nil {
		filtered = []map[string]any{}
	}

	if strings.EqualFold(options.Output, "json") {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(filtered)
	}
	if options.Output != "" && !strings.EqualFold(options.Output, "table") {
		return fmt.Errorf("unsupported output format %q (choose table or json)", options.Output)
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if len(filtered) == 0 {
		fmt.Fprintln(writer, "No results found.")
		return writer.Flush()
	}
	for i, column := range columns {
		if i > 0 {
			fmt.Fprint(writer, "\t")
		}
		fmt.Fprint(writer, column.Title)
	}
	fmt.Fprintln(writer)
	for i := 0; i < len(columns); i++ {
		if i > 0 {
			fmt.Fprint(writer, "\t")
		}
		fmt.Fprint(writer, strings.Repeat("-", len(columns[i].Title)))
	}
	fmt.Fprintln(writer)
	for _, row := range filtered {
		for i, column := range columns {
			if i > 0 {
				fmt.Fprint(writer, "\t")
			}
			fmt.Fprint(writer, displayListValue(row[column.Key]))
		}
		fmt.Fprintln(writer)
	}
	return writer.Flush()
}

func listColumnKeys(columns []ListColumn) string {
	keys := make([]string, 0, len(columns))
	for _, column := range columns {
		keys = append(keys, column.Key)
	}
	return strings.Join(keys, ", ")
}

func compareListValues(left, right any) int {
	switch l := left.(type) {
	case int:
		if r, ok := right.(int); ok {
			if l < r {
				return -1
			}
			if l > r {
				return 1
			}
			return 0
		}
	case int32:
		if r, ok := right.(int32); ok {
			if l < r {
				return -1
			}
			if l > r {
				return 1
			}
			return 0
		}
	case time.Time:
		if r, ok := right.(time.Time); ok {
			if l.Before(r) {
				return -1
			}
			if l.After(r) {
				return 1
			}
			return 0
		}
	}
	return strings.Compare(strings.ToLower(fmt.Sprint(left)), strings.ToLower(fmt.Sprint(right)))
}

func displayListValue(value any) string {
	if value == nil {
		return "-"
	}
	if timestamp, ok := value.(time.Time); ok {
		if timestamp.IsZero() {
			return "-"
		}
		return timestamp.UTC().Format("2006-01-02 15:04:05 UTC")
	}
	if text, ok := value.(string); ok && text == "" {
		return "-"
	}
	return fmt.Sprint(value)
}
