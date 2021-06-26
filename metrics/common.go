package metrics

import "regexp"

type injectLineFunc func(line map[string]string)

func keys(m map[string]string) []string {
	l := []string{}
	for k := range m {
		l = append(l, k)
	}
	return l
}

func makeIfMatchMap(m map[string]string) map[string]*regexp.Regexp {
	if m == nil {
		return nil
	}
	var rv = map[string]*regexp.Regexp{}
	for k, v := range m {
		rv[k] = regexp.MustCompile(v)
	}
	return rv
}
