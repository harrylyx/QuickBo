package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
)

// rulesFile is the exported form of the home rows. Import also takes a bare
// array of rules.
type rulesFile struct {
	App     string     `json:"app"`
	Version int        `json:"version"`
	Rules   []homeRule `json:"rules"`
}

const rulesFileVersion = 1

// draftRules is the rule list as the 分类 page shows it, unsaved edits
// included, without touching the drafts.
func (a *App) draftRules() []homeRule {
	a.bindRuleEdits()
	out := cloneRules(a.activeRules())
	for i := range out {
		if i < len(a.ruleNames) {
			if name := strings.TrimSpace(a.ruleNames[i]); name != "" {
				out[i].Name = name
			}
		}
		if i < len(a.ruleWords) {
			out[i].Keywords = parseKeywords(a.ruleWords[i])
		}
	}
	return out
}

func encodeRules(rules []homeRule) ([]byte, error) {
	return json.MarshalIndent(rulesFile{App: "QuickBo", Version: rulesFileVersion, Rules: rules}, "", "  ")
}

// decodeRules reads an exported rules file (or a bare array of rules) and
// returns the usable rules, in order.
func decodeRules(data []byte) ([]homeRule, error) {
	var file rulesFile
	var rules []homeRule
	trimmed := strings.TrimSpace(string(data))
	switch {
	case strings.HasPrefix(trimmed, "["):
		if err := json.Unmarshal([]byte(trimmed), &rules); err != nil {
			return nil, errors.New("不是有效的分类配置：" + err.Error())
		}
	case strings.HasPrefix(trimmed, "{"):
		if err := json.Unmarshal([]byte(trimmed), &file); err != nil {
			return nil, errors.New("不是有效的分类配置：" + err.Error())
		}
		if file.Version > rulesFileVersion {
			return nil, errors.New("这份分类配置来自更新的版本，请先升级 QuickBo")
		}
		rules = file.Rules
	default:
		return nil, errors.New("不是有效的分类配置：需要 JSON")
	}
	// normalizeRules falls back to the defaults when nothing is usable;
	// an import with no usable row is an error instead.
	usable := false
	for _, r := range rules {
		if strings.TrimSpace(r.ID) != "" && strings.TrimSpace(r.Name) != "" && r.ID != hotLaneID {
			usable = true
			break
		}
	}
	if !usable {
		return nil, errors.New("文件里没有可用的分类（每个分类需要 id 和 name）")
	}
	return normalizeRules(rules), nil
}

// importRules replaces the rows with imported ones as unsaved edits: the
// user reviews them and clicks 保存, as with any other edit.
func (a *App) importRules(rules []homeRule) {
	a.homeRules = cloneRules(rules)
	a.resetRuleEdits()
	a.rulesDirty = true
}

func (a *App) exportRulesDialog() {
	data, err := encodeRules(a.draftRules())
	if err != nil {
		a.rulesNote = "导出失败：" + err.Error()
		return
	}
	win := a.win
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			Title:       "导出首页分类",
			DefaultPath: "QuickBo-首页分类.json",
			Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
		})
		if err != nil || path == "" {
			return
		}
		if filepath.Ext(path) == "" {
			path += ".json"
		}
		err = os.WriteFile(path, data, 0o644)
		a.post(func() {
			if err != nil {
				a.rulesNote = "导出失败：" + err.Error()
				return
			}
			a.rulesNote = "已导出到 " + path
		})
	}()
}

func (a *App) importRulesDialog() {
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  win,
			Title:   "导入首页分类",
			Filters: []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json", "txt"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		var rules []homeRule
		if err == nil {
			rules, err = decodeRules(data)
		}
		a.post(func() {
			if err != nil {
				a.rulesNote = "导入失败：" + err.Error()
				return
			}
			a.importRules(rules)
			a.rulesNote = "已导入 " + strconv.Itoa(len(rules)) + " 个分类，检查无误后点「保存」生效"
		})
	}()
}
