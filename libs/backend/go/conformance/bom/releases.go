package bom

import (
	"cmp"
	"strconv"
	"strings"
)

// LatestRelease escolhe a maior semver pela precedência de semver.org, em que
// 0.10.0 vem depois de 0.9.0 e 1.0.0-rc.1 antes de 1.0.0. O que não é semver
// fica de fora.
func LatestRelease(releases []string) (string, bool) {
	var latest string
	for _, r := range releases {
		if !semverRe.MatchString(r) {
			continue
		}
		if latest == "" || compareSemver(r, latest) > 0 {
			latest = r
		}
	}
	return latest, latest != ""
}

func compareSemver(a, b string) int {
	coreA, preA := splitSemver(a)
	coreB, preB := splitSemver(b)
	for i := range coreA {
		if c := cmp.Compare(coreA[i], coreB[i]); c != 0 {
			return c
		}
	}
	switch {
	case preA == "" && preB == "":
		return 0
	case preA == "":
		return 1
	case preB == "":
		return -1
	}
	idsA, idsB := strings.Split(preA, "."), strings.Split(preB, ".")
	for i := 0; i < len(idsA) && i < len(idsB); i++ {
		numA, errA := strconv.Atoi(idsA[i])
		numB, errB := strconv.Atoi(idsB[i])
		switch {
		case errA == nil && errB == nil:
			if c := cmp.Compare(numA, numB); c != 0 {
				return c
			}
		case errA == nil:
			return -1
		case errB == nil:
			return 1
		default:
			if c := strings.Compare(idsA[i], idsB[i]); c != 0 {
				return c
			}
		}
	}
	return cmp.Compare(len(idsA), len(idsB))
}

func splitSemver(v string) ([3]int, string) {
	v, _, _ = strings.Cut(v, "+")
	core, pre, _ := strings.Cut(v, "-")
	var out [3]int
	for i, f := range strings.SplitN(core, ".", 3) {
		out[i], _ = strconv.Atoi(f)
	}
	return out, pre
}
