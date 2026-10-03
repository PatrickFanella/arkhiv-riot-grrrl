package recovery

import "git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/config"

func writeJSON(path string, v any) error { return config.SaveJSON(path, v) }
