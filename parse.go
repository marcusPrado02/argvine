package argvine

import (
	"fmt"
	"strconv"
)



type flagIndex struct {
	byName map[string]Flag
	byShort map[string]Flag
}


func newFlagIndex() flagIndex {
	return flagIndex{
		byName: make(map[string]Flag),
		byShort: make(map[string]Flag),
	}
}


func (x flagIndex) add(flags []Flag) {
	for _, f := range flags {
		x.byName[f.Name] = f
		if f.Short != "" {
			x.byShort[f.Short] = f
		}
	}
}

func convert(f Flag, raw string) (any, error) {
	switch f.Type {
	case String:
			return raw, nil
	case Int:
			n, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid value %q for --%s: expected an integer", raw, f.Name)
			}
			return n, nil

	case Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q for --%s: expected true or false", raw, f.Name)
		}
		return b, nil
	default:
		panic(fmt.Sprintf("argvine: flag --%s has unknown type %d", f.Name, f.Type))
	}
}