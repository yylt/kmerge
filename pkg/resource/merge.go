package resource

import (
	"encoding/json"

	"dario.cat/mergo"
	"github.com/yylt/kmerge/pkg/util"
	"gopkg.in/yaml.v3"
)

type Mergefn func(s [][]byte) ([]byte, error)

var (
	mergeOpt = []func(*mergo.Config){
		mergo.WithOverride,
		mergo.WithSliceDeepCopy,
	}
)

func TextMerge(s [][]byte) ([]byte, error) {
	buf := util.GetBuf()
	defer util.PutBuf(buf)
	for _, v := range s {
		buf.Write(v)
	}
	return buf.Bytes(), nil
}

func JsonMerge(s [][]byte) ([]byte, error) {
	var (
		data = make([]map[string]any, len(s))
		err  error

		tmp any
	)
	for i, v := range s {
		data[i] = map[string]any{}
		err = json.Unmarshal(v, data[i])
		if err != nil {
			return nil, err
		}
		if i == 0 {
			tmp = data[i]
		}
	}
	for i := 1; i < len(data); i++ {
		err = mergo.Merge(tmp, data[i], mergeOpt...)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(tmp)
}

func YamlMerge(s [][]byte) ([]byte, error) {
	var (
		data = make([]map[string]any, len(s))
		err  error

		tmp any
	)
	for i, v := range s {
		data[i] = map[string]any{}
		err = yaml.Unmarshal(v, data[i])
		if err != nil {
			return nil, err
		}
		if i == 0 {
			tmp = data[i]
		}
	}
	for i := 1; i < len(data); i++ {
		err = mergo.Merge(tmp, data[i], mergeOpt...)
		if err != nil {
			return nil, err
		}
	}
	return yaml.Marshal(tmp)
}
