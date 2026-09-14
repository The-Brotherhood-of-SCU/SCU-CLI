package api

import "testing"

func TestTryParseZNodes(t *testing.T) {
	nodes, matched, err := tryParseZNodes(`<script>var zNodes = [{"id":"1","name":"根"}];</script>`)
	if err != nil || !matched || len(nodes) != 1 {
		t.Fatalf("nodes=%v matched=%v err=%v", nodes, matched, err)
	}
	// 选择页（无 zNodes）：matched=false，不是错误。
	_, matched, err = tryParseZNodes(`<html><button>选择方案</button></html>`)
	if err != nil || matched {
		t.Fatalf("matched=%v err=%v", matched, err)
	}
	// 匹配到但 JSON 损坏：可诊断错误。
	_, matched, err = tryParseZNodes(`var zNodes = [{broken];`)
	if err == nil || !matched {
		t.Fatalf("matched=%v err=%v", matched, err)
	}
}

func TestExtractPlanName(t *testing.T) {
	html := `<script>legend: { data: ['计算机科学与技术培养方案'] }</script>`
	if got := extractPlanName(html); got != "计算机科学与技术培养方案" {
		t.Fatalf("name = %q", got)
	}
	if got := extractPlanName(`<html></html>`); got != "" {
		t.Fatalf("无 legend 时应为空串，得到 %q", got)
	}
}

func TestExtractPlanLinks(t *testing.T) {
	// 1) 按钮形态（onclick 引号为 &#39; 实体 + title 含方案名与 ID）。
	html := `<div><button type="button" title="广播电视编导培养方案(10692)" onclick="getPyfaIndex(&#39;10692&#39;);window.location='/x'">进入</button>` +
		`<button title="计算机培养方案(10693)" onclick="getPyfaIndex(&#39;10693&#39;);">进入</button></div>`
	links := extractPlanLinks(html)
	if len(links) != 2 {
		t.Fatalf("links = %+v", links)
	}
	if links[0].ID != "10692" || links[0].Name != "广播电视编导培养方案" || links[0].Path != "/student/integratedQuery/planCompletion/getPyfaIndex/10692" {
		t.Fatalf("links[0] = %+v", links[0])
	}
	if links[1].ID != "10693" || links[1].Name != "计算机培养方案" {
		t.Fatalf("links[1] = %+v", links[1])
	}

	// 2) 链接形态兜底。
	links = extractPlanLinks(`<a href="/student/integratedQuery/planCompletion/getPyfaIndex/2001?x=1">数学培养方案</a>`)
	if len(links) != 1 || links[0].ID != "2001" || links[0].Name != "数学培养方案" {
		t.Fatalf("anchor links = %+v", links)
	}

	// 3) 裸 ID 兜底 + 去重。
	links = extractPlanLinks(`var url = "getPyfaIndex/3001"; re = getPyfaIndex/3001`)
	if len(links) != 1 || links[0].ID != "3001" || links[0].Name != "方案3001" {
		t.Fatalf("bare links = %+v", links)
	}

	// 无任何形态。
	if links = extractPlanLinks(`<html>empty</html>`); len(links) != 0 {
		t.Fatalf("无入口应返回空，得到 %+v", links)
	}
}
