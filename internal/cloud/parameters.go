package cloud

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

// The current template owns the parameter list; removed parameters are not replayed on updates.
func updateParameters(template string, previous []cf.Parameter, updates map[string]string) ([]cf.Parameter, error) {
	var definition struct{ Parameters map[string]json.RawMessage }
	if err := json.Unmarshal([]byte(template), &definition); err != nil {
		return nil, err
	}
	for key := range updates {
		if _, ok := definition.Parameters[key]; !ok {
			return nil, fmt.Errorf("unknown template parameter %s", key)
		}
	}
	existing := map[string]bool{}
	for _, p := range previous {
		existing[aws.ToString(p.ParameterKey)] = true
	}
	keys := make([]string, 0, len(definition.Parameters))
	for k := range definition.Parameters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := []cf.Parameter{}
	for _, key := range keys {
		if value, ok := updates[key]; ok {
			result = append(result, cf.Parameter{ParameterKey: aws.String(key), ParameterValue: aws.String(value)})
		} else if existing[key] {
			result = append(result, cf.Parameter{ParameterKey: aws.String(key), UsePreviousValue: aws.Bool(true)})
		}
	}
	return result, nil
}
