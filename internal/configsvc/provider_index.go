package configsvc

import "meno/internal/llm"

func providerIndex(ps []llm.Provider, name string) int {
	for i := range ps {
		if ps[i].Name == name {
			return i
		}
	}
	return -1
}
