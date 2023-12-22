package resource

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

var _ sort.Interface = &seInfos{}

type seInfo struct {
	*corev1.Secret
}

type seInfos []seInfo

func (se seInfos) Len() int {
	return len(se)
}

// TODO use index annotation to compare
func (se seInfos) Less(i, j int) bool {
	n1 := se[i]
	n2 := se[j]

	return n1.Namespace+n1.Name < n2.Namespace+n2.Name
}

func (se seInfos) Swap(i, j int) {
	se[i], se[j] = se[j], se[i]
}
