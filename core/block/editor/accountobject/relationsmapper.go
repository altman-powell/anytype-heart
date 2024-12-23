package accountobject

import (
	"fmt"

	"github.com/anyproto/any-store/anyenc"
	"github.com/gogo/protobuf/types"

	"github.com/anyproto/anytype-heart/core/domain"
)

type KeyType int

const (
	KeyTypeString KeyType = iota
	KeyTypeInt64
)

type relationsMapper struct {
	keys map[string]KeyType
}

func newRelationsMapper(keys map[string]KeyType) *relationsMapper {
	return &relationsMapper{
		keys: keys,
	}
}

func (r *relationsMapper) GetRelationKey(key string, val *anyenc.Value) (domain.Value, bool) {
	kt, ok := r.keys[key]
	if !ok {
		return domain.Invalid(), false
	}
	switch kt {
	case KeyTypeString:
		val := val.GetStringBytes(key)
		if val == nil {
			return domain.Invalid(), false
		}
		return domain.String(string(val)), true
	case KeyTypeInt64:
		val := val.GetInt(key)
		if val == 0 {
			return domain.Invalid(), false
		}
		return domain.Int64(int64(val)), true
	}
	return domain.Invalid(), false
}

func (r *relationsMapper) GetStoreKey(key string, val *types.Value) (res any, ok bool) {
	kt, ok := r.keys[key]
	if !ok {
		return nil, false
	}
	switch kt {
	case KeyTypeString:
		res = val.GetStringValue()
		if res == "" {
			return nil, false
		}
		res = fmt.Sprintf(`"%s"`, res)
	case KeyTypeInt64:
		res = int64(val.GetNumberValue())
		if res == 0 {
			return nil, false
		}
	}
	return res, true
}
