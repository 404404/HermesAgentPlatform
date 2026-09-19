package main

import "testing"

func TestCompletionParseQAHandlesBOMQuotedNewlinesAndTags(t *testing.T) {
	rows, problems := completionParseQA(completionQAImportRequest{CSV: "\ufeffquestion,answer,tags\n\"How does this work?\",\"First line\\nSecond line\",policy|security\n"})
	if len(problems) != 0 {
		t.Fatalf("unexpected parse problems: %#v", problems)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if rows[0].Answer != "First line\nSecond line" {
		t.Fatalf("quoted newline was not preserved: %q", rows[0].Answer)
	}
	if len(rows[0].Tags) != 2 || rows[0].Tags[0] != "policy" || rows[0].Tags[1] != "security" {
		t.Fatalf("tags were not normalized: %#v", rows[0].Tags)
	}
}

func TestCompletionParseQARejectsMissingColumns(t *testing.T) {
	_, problems := completionParseQA(completionQAImportRequest{CSV: "title,content\nA,B\n"})
	if len(problems) != 1 || problems[0]["error"] != "question_and_answer_columns_required" {
		t.Fatalf("expected required-column error, got %#v", problems)
	}
}

func TestCompletionIDsAndTagsAreDeterministic(t *testing.T) {
	ids := completionIDs([]int64{3, 0, 2, 3, -1})
	if len(ids) != 2 || ids[0] != 2 || ids[1] != 3 {
		t.Fatalf("unexpected IDs: %#v", ids)
	}
	tags := completionTags([]string{" policy | security ", "security", "runtime"})
	if len(tags) != 3 || tags[0] != "policy" || tags[1] != "security" || tags[2] != "runtime" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
}
