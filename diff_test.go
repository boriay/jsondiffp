package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestDiffObjectField(t *testing.T) {
	diffs := diffFromJSON(t, `{"name":"old","enabled":true}`, `{"name":"new","enabled":true}`)
	assertDiffs(t, diffs, []DiffEntry{{Path: "$.name", Type: "changed", Left: "old", Right: "new"}})
}

func TestDiffArrayElement(t *testing.T) {
	diffs := diffFromJSON(t, `{"values":[1,2,3]}`, `{"values":[1,20,3]}`)
	assertDiffs(t, diffs, []DiffEntry{{Path: "$.values[1]", Type: "changed", Left: json.Number("2"), Right: json.Number("20")}})
}

func TestDiffNumberPrecision(t *testing.T) {
	left := decodeJSON(t, `{"values":[1.24,1.25,1.26,1.24]}`)
	right := decodeJSON(t, `{"values":[1.2,1.3,1.3,1.3]}`)

	assertDiffs(t, DiffJSONWithPrecision(left, right, 1), []DiffEntry{{Path: "$.values[3]", Type: "changed", Left: json.Number("1.24"), Right: json.Number("1.3")}})
	assertDiffs(t, DiffJSONWithPrecision(left, right, 0), nil)
	assertDiffs(t, DiffJSONWithOptions(left, right, DiffOptions{Precision: 2, RoundMode: RoundDirect}), []DiffEntry{
		{Path: "$.values[0]", Type: "changed", Left: json.Number("1.24"), Right: json.Number("1.2")},
		{Path: "$.values[1]", Type: "changed", Left: json.Number("1.25"), Right: json.Number("1.3")},
		{Path: "$.values[2]", Type: "changed", Left: json.Number("1.26"), Right: json.Number("1.3")},
		{Path: "$.values[3]", Type: "changed", Left: json.Number("1.24"), Right: json.Number("1.3")},
	})
}

func TestDiffNumberPrecisionRoundsNumbers(t *testing.T) {
	assertDiffs(t, DiffJSONWithPrecision(decodeJSON(t, `{"value":289.24826}`), decodeJSON(t, `{"value":289.25018}`), 1), nil)
	assertDiffs(t, DiffJSONWithOptions(decodeJSON(t, `{"value":289.24826}`), decodeJSON(t, `{"value":289.25018}`), DiffOptions{Precision: 1, RoundMode: RoundDirect}), []DiffEntry{{Path: "$.value", Type: "changed", Left: json.Number("289.24826"), Right: json.Number("289.25018")}})
	assertDiffs(t, DiffJSONWithPrecision(decodeJSON(t, `{"value":289.25018}`), decodeJSON(t, `{"value":289.25118}`), 1), nil)
	assertDiffs(t, DiffJSONWithPrecision(decodeJSON(t, `{"value":289.24426}`), decodeJSON(t, `{"value":289.2}`), 1), nil)
	assertDiffs(t, DiffJSONWithPrecision(decodeJSON(t, `{"value":-1.5}`), decodeJSON(t, `{"value":-2}`), 0), nil)
	assertDiffs(t, DiffJSONWithPrecision(decodeJSON(t, `{"value":-1.49}`), decodeJSON(t, `{"value":-1}`), 0), []DiffEntry{{Path: "$.value", Type: "changed", Left: json.Number("-1.49"), Right: json.Number("-1")}})
	assertDiffs(t, DiffJSONWithOptions(decodeJSON(t, `{"value":-1.49}`), decodeJSON(t, `{"value":-1}`), DiffOptions{Precision: 0, RoundMode: RoundDirect}), nil)
}

func TestDiffNumberPrecisionKeepsLargeNumbersExact(t *testing.T) {
	assertDiffs(t, DiffJSONWithPrecision(decodeJSON(t, `{"value":9007199254740992}`), decodeJSON(t, `{"value":9007199254740993}`), -1), []DiffEntry{{Path: "$.value", Type: "changed", Left: json.Number("9007199254740992"), Right: json.Number("9007199254740993")}})
	assertDiffs(t, DiffJSONWithOptions(decodeJSON(t, `{"value":9007199254740992.24}`), decodeJSON(t, `{"value":9007199254740992.23}`), DiffOptions{Precision: 1, RoundMode: RoundDirect}), nil)
}

func TestDiffNestedObject(t *testing.T) {
	diffs := diffFromJSON(t, `{"profile":{"address":{"city":"Moscow"}}}`, `{"profile":{"address":{"city":"SPB"}}}`)
	assertDiffs(t, diffs, []DiffEntry{{Path: "$.profile.address.city", Type: "changed", Left: "Moscow", Right: "SPB"}})
}

func TestDiffObjectFieldsAddedAndRemoved(t *testing.T) {
	diffs := diffFromJSON(t, `{"id":1,"oldField":"gone"}`, `{"id":1,"newField":"added"}`)
	assertDiffs(t, diffs, []DiffEntry{
		{Path: "$.newField", Type: "added", Right: "added"},
		{Path: "$.oldField", Type: "removed", Left: "gone"},
	})
}

func TestDiffNestedObjectInsideArray(t *testing.T) {
	diffs := diffFromJSON(t, `{"users":[{"id":1,"meta":{"active":true}}]}`, `{"users":[{"id":1,"meta":{"active":false}}]}`)
	assertDiffs(t, diffs, []DiffEntry{{Path: "$.users[0].meta.active", Type: "changed", Left: true, Right: false}})
}

func TestDiffArrayLengthAndObjectDifference(t *testing.T) {
	diffs := diffFromJSON(t,
		`{"items":[{"id":1,"value":"a"},{"id":2,"value":"b"}]}`,
		`{"items":[{"id":1,"value":"A"},{"id":2,"value":"b"},{"id":3,"value":"c", "arr":[1,2,3]}]}`,
	)
	assertDiffs(t, diffs, []DiffEntry{
		{Path: "$.items[0].value", Type: "changed", Left: "a", Right: "A"},
		{Path: "$.items[2]", Type: "added", Right: map[string]any{"arr": []any{json.Number("1"), json.Number("2"), json.Number("3")}, "id": json.Number("3"), "value": "c"}},
	})
}

func TestDiffTypeChangeFallsBackToHigherLevel(t *testing.T) {
	diffs := diffFromJSON(t, `{"payload":{"value":1}}`, `{"payload":[{"value":1}]}`)
	assertDiffs(t, diffs, []DiffEntry{{Path: "$.payload", Type: "type_changed", Left: map[string]any{"value": json.Number("1")}, Right: []any{map[string]any{"value": json.Number("1")}}}})
}

func TestDiffJSONOutputKeepsNullValues(t *testing.T) {
	diffs := diffFromJSON(t, `{"value":null}`, `{"value":1}`)
	output, err := json.Marshal(diffs)
	if err != nil {
		t.Fatalf("marshal diff: %v", err)
	}

	want := `[{"path":"$.value","type":"type_changed","left":null,"right":1}]`
	if string(output) != want {
		t.Fatalf("unexpected json output\nwant: %s\n got: %s", want, output)
	}
}

func TestWriteTextDiff(t *testing.T) {
	diffs := []DiffEntry{
		{Path: "$.name", Type: "changed", Left: "old", Right: "new"},
		{Path: "$.profile.city", Type: "changed", Left: "Moscow", Right: "SPB"},
		{Path: "$.profile.zip", Type: "changed", Left: json.Number("1"), Right: json.Number("2")},
		{Path: "$.items[1]", Type: "removed", Left: map[string]any{"id": json.Number("2")}},
		{Path: "$.items[2]", Type: "added", Right: map[string]any{"id": json.Number("3")}},
	}

	var buffer bytes.Buffer
	if err := WriteTextDiff(&buffer, diffs); err != nil {
		t.Fatalf("write text diff: %v", err)
	}

	want := "$\n" +
		"  changed .name\n" +
		"    left: \"old\"\n" +
		"    right: \"new\"\n" +
		"\n" +
		"$.profile\n" +
		"  changed .city\n" +
		"    left: \"Moscow\"\n" +
		"    right: \"SPB\"\n" +
		"  changed .zip\n" +
		"    left: 1\n" +
		"    right: 2\n" +
		"\n" +
		"$.items\n" +
		"  removed [1]\n" +
		"    left: {\"id\":2}\n" +
		"  added [2]\n" +
		"    right: {\"id\":3}\n"
	if buffer.String() != want {
		t.Fatalf("unexpected text diff\nwant:\n%s\ngot:\n%s", want, buffer.String())
	}
}

func TestWriteTextDiffWithoutDifferences(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteTextDiff(&buffer, nil); err != nil {
		t.Fatalf("write text diff: %v", err)
	}

	if buffer.String() != "no differences\n" {
		t.Fatalf("unexpected text diff: %q", buffer.String())
	}
}

func TestCLIPrintsDiff(t *testing.T) {
	tmp := t.TempDir()
	leftPath := tmp + "/left.json"
	rightPath := tmp + "/right.json"
	mustWriteFile(t, leftPath, `{"items":[{"name":"a"}]}`)
	mustWriteFile(t, rightPath, `{"items":[{"name":"b"}]}`)

	cmd := exec.Command("go", "run", ".", leftPath, rightPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}

	want := "$.items[0]\n" +
		"  changed .name\n" +
		"    left: \"a\"\n" +
		"    right: \"b\"\n"
	if string(output) != want {
		t.Fatalf("unexpected cli output\nwant:\n%s\ngot:\n%s", want, output)
	}
}

func TestCLIPrintsJSONDiff(t *testing.T) {
	tmp := t.TempDir()
	leftPath := tmp + "/left.json"
	rightPath := tmp + "/right.json"
	mustWriteFile(t, leftPath, `{"items":[{"name":"a"}]}`)
	mustWriteFile(t, rightPath, `{"items":[{"name":"b"}]}`)

	cmd := exec.Command("go", "run", ".", "-j", leftPath, rightPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}

	var diffs []DiffEntry
	if err := json.Unmarshal(output, &diffs); err != nil {
		t.Fatalf("unmarshal cli output: %v\n%s", err, output)
	}
	assertDiffs(t, diffs, []DiffEntry{{Path: "$.items[0].name", Type: "changed", Left: "a", Right: "b"}})
}

func TestCLIJSONDiffUsesNumberPrecision(t *testing.T) {
	tmp := t.TempDir()
	leftPath := tmp + "/left.json"
	rightPath := tmp + "/right.json"
	mustWriteFile(t, leftPath, `{"value":289.24826}`)
	mustWriteFile(t, rightPath, `{"value":289.25018}`)

	cmd := exec.Command("go", "run", ".", "-p", "1", "-j", leftPath, rightPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}

	var diffs []DiffEntry
	if err := json.Unmarshal(output, &diffs); err != nil {
		t.Fatalf("unmarshal cli output: %v\n%s", err, output)
	}
	assertDiffs(t, diffs, nil)
}

func TestCLIJSONDiffUsesDirectRound(t *testing.T) {
	tmp := t.TempDir()
	leftPath := tmp + "/left.json"
	rightPath := tmp + "/right.json"
	mustWriteFile(t, leftPath, `{"value":289.24826}`)
	mustWriteFile(t, rightPath, `{"value":289.25018}`)

	cmd := exec.Command("go", "run", ".", "-p", "1", "-r", "-j", leftPath, rightPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}

	var diffs []DiffEntry
	if err := json.Unmarshal(output, &diffs); err != nil {
		t.Fatalf("unmarshal cli output: %v\n%s", err, output)
	}
	assertDiffs(t, diffs, []DiffEntry{{Path: "$.value", Type: "changed", Left: 289.24826, Right: 289.25018}})
}

func TestCLIUsesNumberPrecision(t *testing.T) {
	tmp := t.TempDir()
	leftPath := tmp + "/left.json"
	rightPath := tmp + "/right.json"
	mustWriteFile(t, leftPath, `{"value":1.24}`)
	mustWriteFile(t, rightPath, `{"value":1.2}`)

	cmd := exec.Command("go", "run", ".", "-p", "1", leftPath, rightPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}

	if string(output) != "no differences\n" {
		t.Fatalf("unexpected cli output: %s", output)
	}
}

func TestCLIRejectsNegativeNumberPrecision(t *testing.T) {
	tmp := t.TempDir()
	leftPath := tmp + "/left.json"
	rightPath := tmp + "/right.json"
	mustWriteFile(t, leftPath, `{"value":1}`)
	mustWriteFile(t, rightPath, `{"value":1}`)

	cmd := exec.Command("go", "run", ".", "-p", "-1", leftPath, rightPath)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected go run to fail, output: %s", output)
	}
	if !strings.Contains(string(output), "precision must be non-negative") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestReadJSONFileRejectsExtraValue(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/broken.json"
	mustWriteFile(t, path, `{"ok":true} {"extra":true}`)

	_, err := readJSONFile(path)
	if err == nil {
		t.Fatal("expected error for extra json value")
	}
}

func diffFromJSON(t *testing.T, leftRaw, rightRaw string) []DiffEntry {
	t.Helper()
	return DiffJSON(decodeJSON(t, leftRaw), decodeJSON(t, rightRaw))
}

func decodeJSON(t *testing.T, raw string) any {
	t.Helper()
	var value any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return value
}

func assertDiffs(t *testing.T, got, want []DiffEntry) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected diffs\nwant: %#v\n got: %#v", want, got)
	}
}

func mustWriteFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
