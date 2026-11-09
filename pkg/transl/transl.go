package transl

import (
	"context"
	"fmt"
	"larsa-academy-microservice/pkg/util"

	"github.com/goccy/go-json"
)

type Localizable[T any] map[string]T

func (l Localizable[T]) MarshalJSON(ctx context.Context) ([]byte, error) {
	if l == nil {
		return []byte("null"), nil
	}

	safeMap := make(map[string]T, len(l))
	for k, v := range l {
		safeMap[k] = v
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return json.Marshal(safeMap)
	}

	lang := cfg.Lang
	if lang == "" {
		return json.Marshal(safeMap)
	}

	content, exists := safeMap[lang]
	if exists {
		if isEmptyOrNil(any(content)) {
			return []byte("null"), nil
		}
		return json.Marshal(content)
	}

	enContent := safeMap["en"]
	if isEmptyOrNil(any(enContent)) {
		return []byte("null"), nil
	}
	return json.Marshal(enContent)
}

func isEmptyOrNil(v any) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return x == ""
	case *string:
		return x == nil
	}
	return false
}

func (l *Localizable[T]) UnmarshalJSON(ctx context.Context, data []byte) error {
	var result map[string]T
	err := json.Unmarshal(data, &result)

	if err != nil {
		var t T
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		result = map[string]T{
			"en": t,
		}
	}

	*l = result

	return nil
}

func (l Localizable[T]) GetContentByLang(lang string) T {
	if l == nil {
		var zero T
		return zero
	}
	content, exists := l[lang]
	if exists {
		return content
	} else {
		return l["en"]
	}
}

// GetContentByContext gets content using language from context (Accept-Language header)
// Falls back to "en" if language is not found or context error occurs
func (l Localizable[T]) GetContentByContext(ctx context.Context) T {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		// Fallback to default language
		return l.GetContentByLang("en")
	}

	lang := cfg.Lang
	if lang == "" {
		lang = "en"
	}

	return l.GetContentByLang(lang)
}

func MergeLocalizable[T any](m1, m2 Localizable[T], separator string) Localizable[string] {
	result := make(Localizable[string])
	for lang, v1 := range m1 {
		v2 := m2[lang]
		result[lang] = fmt.Sprintf("%v%s%v", v1, separator, v2)
	}
	return result
}
