package plan9asm

import (
	"fmt"
	"sort"
	"strings"
)

const featureAttrBase = 200

type featureAttrRegistry struct {
	order []string
	refs  map[string]string
}

func newFeatureAttrRegistry() *featureAttrRegistry {
	return &featureAttrRegistry{refs: make(map[string]string)}
}

func (r *featureAttrRegistry) ref(features string) string {
	if features == "" {
		return ""
	}
	if ref, ok := r.refs[features]; ok {
		return ref
	}
	ref := fmt.Sprintf("#%d", featureAttrBase+len(r.order))
	r.order = append(r.order, features)
	r.refs[features] = ref
	return ref
}

func (r *featureAttrRegistry) emit(b *strings.Builder) {
	if len(r.order) == 0 {
		return
	}
	for _, features := range r.order {
		fmt.Fprintf(b, "attributes %s = { \"target-features\"=%q }\n", r.refs[features], features)
	}
	b.WriteString("\n")
}

func inferFuncTargetFeatures(arch Arch, fn Func) string {
	var featureSet []string
	add := func(features ...string) {
		for _, feature := range features {
			if feature == "" {
				continue
			}
			exists := false
			for _, v := range featureSet {
				if v == feature {
					exists = true
					break
				}
			}
			if !exists {
				featureSet = append(featureSet, feature)
			}
		}
	}

	for _, ins := range fn.Instrs {
		op := strings.ToUpper(string(ins.Op))
		switch arch {
		case ArchAMD64:
			switch {
			case strings.HasPrefix(op, "CRC32"):
				add("+crc32", "+sse4.2")
			case op == "PCLMULQDQ":
				add("+pclmul", "+sse4.1")
			case op == "PSHUFB" || op == "VPSHUFB":
				add("+ssse3")
			case op == "AESENC" || op == "AESENCLAST" || op == "AESDEC" || op == "AESDECLAST" || op == "AESIMC" || op == "AESKEYGENASSIST":
				add("+aes")
			}
		case ArchARM64:
			if strings.HasPrefix(op, "CRC32") {
				add("+crc")
			}
		}
	}

	if len(featureSet) == 0 {
		return ""
	}
	sort.Strings(featureSet)
	return strings.Join(featureSet, ",")
}
