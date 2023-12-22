package pkg

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

const (
	// text type, support text, json, yaml. default is text
	KmergeTypeKey = "kmerge.io/type"

	// primary resource, other data will be merged into here
	KmergePrimaryKey = "kmerge.io/primary"

	// resource name which samed will merged
	KmergeNameKey = "kmerge.io/name"

	// merge resource from which namespace
	KmergeFromNsKey = "namespace.kmerge.io/from"

	// resource to which namespace
	KmergeToNsKey = "namespace.kmerge.io/to"

	KmergeHashKey = "kmerge.io/hash"
)

type Kind string

const (
	Textk Kind = "text"
	Jsonk Kind = "json"
	Yamlk Kind = "yaml"
)

func ValidKind(k string) (Kind, bool) {
	switch k {
	case string(Textk):
		return Textk, true
	case string(Jsonk):
		return Jsonk, true
	case string(Yamlk):
		return Yamlk, true
	default:
		return Textk, false
	}
}

func (k Kind) Unmarshal(d []byte, v any) error {
	switch k {
	case Textk:
		v = d
		return nil
	case Jsonk:
		return json.Unmarshal(d, v)
	case Yamlk:
		return yaml.Unmarshal(d, v)

	default:
		return fmt.Errorf("not support")
	}
}

func (k Kind) Marshal(v any) ([]byte, error) {
	switch k {
	case Textk:
		return v.([]byte), nil
	case Jsonk:
		return json.Marshal(v)
	case Yamlk:
		return yaml.Marshal(v)
	default:
		return nil, fmt.Errorf("not support")
	}
}
