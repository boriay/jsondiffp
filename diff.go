package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"regexp"
	"sort"
	"strconv"
)

type DiffEntry struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	Left  any    `json:"left"`
	Right any    `json:"right"`
}

type RoundMode int

const (
	RoundAccumulated RoundMode = iota
	RoundDirect
)

type DiffOptions struct {
	Precision int
	RoundMode RoundMode
}

func WriteTextDiff(writer io.Writer, diffs []DiffEntry) error {
	if len(diffs) == 0 {
		_, err := fmt.Fprintln(writer, "no differences")
		return err
	}

	currentGroup := ""
	for index, diff := range diffs {
		groupPath := diffGroupPath(diff.Path)
		if groupPath != currentGroup {
			if index > 0 {
				if _, err := fmt.Fprintln(writer); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "%s\n", groupPath); err != nil {
				return err
			}
			currentGroup = groupPath
		}

		if _, err := fmt.Fprintf(writer, "  %s %s\n", diff.Type, diffRelativePath(groupPath, diff.Path)); err != nil {
			return err
		}

		switch diff.Type {
		case "added":
			if err := writeDiffValue(writer, "right", diff.Right); err != nil {
				return err
			}
		case "removed":
			if err := writeDiffValue(writer, "left", diff.Left); err != nil {
				return err
			}
		default:
			if err := writeDiffValue(writer, "left", diff.Left); err != nil {
				return err
			}
			if err := writeDiffValue(writer, "right", diff.Right); err != nil {
				return err
			}
		}
	}

	return nil
}

func diffGroupPath(path string) string {
	if path == "$" {
		return "$"
	}

	lastDot := -1
	lastBracket := -1
	for index := len(path) - 1; index >= 0; index-- {
		switch path[index] {
		case '.':
			lastDot = index
			index = -1
		case ']':
			if lastBracket == -1 {
				lastBracket = index
			}
		case '[':
			if lastBracket != -1 {
				return path[:index]
			}
		}
	}

	if lastDot > 0 {
		return path[:lastDot]
	}
	return "$"
}

func diffRelativePath(groupPath, path string) string {
	if groupPath == path {
		return "<self>"
	}
	if groupPath == "$" {
		return path[1:]
	}
	return path[len(groupPath):]
}

func writeDiffValue(writer io.Writer, name string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "    %s: %s\n", name, encoded)
	return err
}

func DiffJSON(left, right any) []DiffEntry {
	return DiffJSONWithPrecision(left, right, -1)
}

func DiffJSONWithPrecision(left, right any, precision int) []DiffEntry {
	return DiffJSONWithOptions(left, right, DiffOptions{Precision: precision, RoundMode: RoundAccumulated})
}

func DiffJSONWithOptions(left, right any, options DiffOptions) []DiffEntry {
	diffs := make([]DiffEntry, 0)
	diffValue("$", left, right, options, &diffs)
	return diffs
}

func diffValue(path string, left, right any, options DiffOptions, diffs *[]DiffEntry) {
	leftObject, leftIsObject := left.(map[string]any)
	rightObject, rightIsObject := right.(map[string]any)
	if leftIsObject || rightIsObject {
		if !leftIsObject || !rightIsObject {
			addChanged(path, left, right, diffs)
			return
		}
		diffObject(path, leftObject, rightObject, options, diffs)
		return
	}

	leftArray, leftIsArray := left.([]any)
	rightArray, rightIsArray := right.([]any)
	if leftIsArray || rightIsArray {
		if !leftIsArray || !rightIsArray {
			addChanged(path, left, right, diffs)
			return
		}
		diffArray(path, leftArray, rightArray, options, diffs)
		return
	}

	if !sameScalar(left, right, options) {
		addChanged(path, left, right, diffs)
	}
}

func diffObject(path string, left, right map[string]any, options DiffOptions, diffs *[]DiffEntry) {
	keys := make([]string, 0, len(left)+len(right))
	seen := make(map[string]struct{}, len(left)+len(right))

	for key := range left {
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	for key := range right {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		childPath := appendObjectPath(path, key)
		leftValue, leftOK := left[key]
		rightValue, rightOK := right[key]

		switch {
		case leftOK && rightOK:
			diffValue(childPath, leftValue, rightValue, options, diffs)
		case leftOK:
			*diffs = append(*diffs, DiffEntry{Path: childPath, Type: "removed", Left: leftValue})
		default:
			*diffs = append(*diffs, DiffEntry{Path: childPath, Type: "added", Right: rightValue})
		}
	}
}

func diffArray(path string, left, right []any, options DiffOptions, diffs *[]DiffEntry) {
	sharedLen := min(len(left), len(right))
	for index := 0; index < sharedLen; index++ {
		diffValue(fmt.Sprintf("%s[%d]", path, index), left[index], right[index], options, diffs)
	}

	for index := sharedLen; index < len(left); index++ {
		*diffs = append(*diffs, DiffEntry{Path: fmt.Sprintf("%s[%d]", path, index), Type: "removed", Left: left[index]})
	}
	for index := sharedLen; index < len(right); index++ {
		*diffs = append(*diffs, DiffEntry{Path: fmt.Sprintf("%s[%d]", path, index), Type: "added", Right: right[index]})
	}
}

func addChanged(path string, left, right any, diffs *[]DiffEntry) {
	diffType := "changed"
	if reflect.TypeOf(left) != reflect.TypeOf(right) {
		diffType = "type_changed"
	}
	*diffs = append(*diffs, DiffEntry{Path: path, Type: diffType, Left: left, Right: right})
}

func sameScalar(left, right any, options DiffOptions) bool {
	leftNumber, leftIsNumber := left.(json.Number)
	rightNumber, rightIsNumber := right.(json.Number)
	if leftIsNumber && rightIsNumber {
		return sameJSONNumber(leftNumber, rightNumber, options)
	}

	return reflect.DeepEqual(left, right)
}

func sameJSONNumber(left, right json.Number, options DiffOptions) bool {
	leftValue, leftOK := new(big.Rat).SetString(left.String())
	rightValue, rightOK := new(big.Rat).SetString(right.String())
	if !leftOK || !rightOK {
		return left.String() == right.String()
	}

	if options.Precision >= 0 {
		leftValue = roundJSONNumber(left, leftValue, options)
		rightValue = roundJSONNumber(right, rightValue, options)
	}
	return leftValue.Cmp(rightValue) == 0
}

func roundJSONNumber(number json.Number, value *big.Rat, options DiffOptions) *big.Rat {
	if options.RoundMode == RoundDirect {
		return roundRat(value, options.Precision)
	}

	rounded := new(big.Rat).Set(value)
	fractionDigits := jsonNumberFractionDigits(number.String())
	for currentPrecision := fractionDigits - 1; currentPrecision >= options.Precision; currentPrecision-- {
		rounded = roundRat(rounded, currentPrecision)
	}
	return roundRat(rounded, options.Precision)
}

func jsonNumberFractionDigits(number string) int {
	dotIndex := -1
	endIndex := len(number)
	for index, char := range number {
		switch char {
		case '.':
			dotIndex = index
		case 'e', 'E':
			endIndex = index
		}
	}
	if dotIndex < 0 {
		return 0
	}
	return endIndex - dotIndex - 1
}

func roundRat(value *big.Rat, precision int) *big.Rat {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(precision)), nil)
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(scale))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(scaled.Num(), scaled.Denom(), remainder)

	doubleRemainder := new(big.Int).Mul(new(big.Int).Abs(remainder), big.NewInt(2))
	if doubleRemainder.Cmp(scaled.Denom()) >= 0 {
		if scaled.Num().Sign() >= 0 {
			quotient.Add(quotient, big.NewInt(1))
		} else {
			quotient.Sub(quotient, big.NewInt(1))
		}
	}

	return new(big.Rat).SetFrac(quotient, scale)
}

var plainPathKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func appendObjectPath(path, key string) string {
	if plainPathKey.MatchString(key) {
		return path + "." + key
	}
	return path + "[" + strconv.Quote(key) + "]"
}
