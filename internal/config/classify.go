package config

import "strings"

// Capability is the kind of a site and whether QuickBo drives it.
type Capability struct {
	Kind   string
	Level  Level
	Reason string
}

// Kind labels used by Capability.Kind.
const (
	KindCMSJSON = "cms-json"
	KindCMSXML  = "cms-xml"
	KindJS      = "js"
	KindXBPQ    = "xbpq"
	KindXYQ     = "xyq"
	KindJar     = "jar"
	KindPython  = "python"
	KindXPath   = "xpath"
	KindRemote  = "remote"
	KindUnknown = "unknown"
)

// KindLabel is a short Chinese name for a kind.
func KindLabel(kind string) string {
	switch kind {
	case KindCMSJSON:
		return "JSON CMS"
	case KindCMSXML:
		return "XML CMS"
	case KindJS:
		return "JS / drpy"
	case KindXBPQ:
		return "XBPQ 规则"
	case KindXYQ:
		return "XYQ 规则"
	case KindJar:
		return "JAR/DEX 爬虫"
	case KindPython:
		return "Python 爬虫"
	case KindXPath:
		return "XPath"
	case KindRemote:
		return "远程站点"
	default:
		return "未知"
	}
}

// Classify reports how a site should be driven. JAR/DEX spiders (type 3
// with a csp_* class that is not a pure-Go rule engine, or an api/jar
// that points at a .jar) are detected and marked unsupported. They are
// never executed.
func Classify(s Site) Capability {
	api := stripQuery(strings.TrimSpace(s.API))
	low := strings.ToLower(api)
	ext := stripQuery(s.ExtString())
	extLow := strings.ToLower(ext)
	jar := strings.ToLower(strings.TrimSpace(s.Jar))

	switch s.Type {
	case TypeJSON:
		if isHTTP(s.API) {
			return Capability{Kind: KindCMSJSON, Level: LevelYes}
		}
		return Capability{Kind: KindUnknown, Level: LevelNo, Reason: "JSON CMS 的 api 不是 http(s) 地址"}
	case TypeXML:
		if isHTTP(s.API) {
			return Capability{Kind: KindCMSXML, Level: LevelYes}
		}
		return Capability{Kind: KindXPath, Level: LevelNo, Reason: "type 0 但 api 不是 XML CMS 地址（XPath 规则本版不支持）"}
	case TypeRemote:
		return Capability{Kind: KindRemote, Level: LevelNo, Reason: "type 4 远程站点本版不作为点播源播放"}
	case TypeSpider:
		if strings.HasSuffix(low, ".py") || strings.HasSuffix(extLow, ".py") {
			return Capability{Kind: KindPython, Level: LevelNo, Reason: "Python 爬虫不支持"}
		}
		if strings.HasPrefix(low, "csp_") {
			switch low {
			case "csp_xbpq":
				return Capability{Kind: KindXBPQ, Level: LevelYes}
			case "csp_xyqhiker", "csp_xyqhikeral":
				return Capability{Kind: KindXYQ, Level: LevelPartial, Reason: "按 XBPQ 的截取字段解释，XYQ 专有语法可能不完整"}
			default:
				return Capability{Kind: KindJar, Level: LevelNo, Reason: jarReason(s.API)}
			}
		}
		if strings.HasSuffix(low, ".jar") || strings.HasSuffix(jar, ".jar") || strings.Contains(low, ".jar") {
			return Capability{Kind: KindJar, Level: LevelNo, Reason: jarReason(s.API)}
		}
		if strings.HasSuffix(low, ".js") || strings.HasSuffix(extLow, ".js") || looksInlineJS(s.ExtString()) {
			return Capability{Kind: KindJS, Level: LevelPartial, Reason: "支持 drpy 的 rule 对象，以及 home/category/detail/search/play 函数；不支持完整 drpy2 模板方言"}
		}
		if isHTTP(s.API) {
			return Capability{Kind: KindJS, Level: LevelPartial, Reason: "按 JS 源尝试加载"}
		}
		return Capability{Kind: KindUnknown, Level: LevelNo, Reason: "无法识别的 type 3 爬虫"}
	default:
		if isHTTP(s.API) && (strings.Contains(low, "provide/vod") || strings.Contains(low, "api.php")) {
			return Capability{Kind: KindCMSJSON, Level: LevelPartial, Reason: "type 未标明，按 JSON CMS 尝试"}
		}
		return Capability{Kind: KindUnknown, Level: LevelNo, Reason: "未知的站点 type"}
	}
}

func jarReason(api string) string {
	return "JAR/DEX 爬虫（" + strings.TrimSpace(api) + "）本版不执行。QuickBo 不加载 jar，也不会运行 csp_* Android 类。"
}

func stripQuery(s string) string {
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		return s[:i]
	}
	return s
}

func isHTTP(s string) bool {
	l := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

func looksInlineJS(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "var rule") || strings.HasPrefix(t, "function ") || strings.HasPrefix(t, "export ")
}
