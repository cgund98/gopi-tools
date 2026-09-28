package linear

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gopi"
)

// commentsOps are the operations linear_comments exposes. Both are writes, so
// the tool is registered in agent mode only.
var commentsOps = ops{
	{name: "create_comment", write: true},
	{name: "update_comment", write: true},
}

// commentsArgs holds the arguments linear_comments accepts.
type commentsArgs struct {
	Operation string `json:"operation" jsonschema_description:"The operation to perform."`

	Issue     string `json:"issue,omitempty" jsonschema_description:"Issue identifier such as ENG-123, or a UUID. Required for create_comment."`
	CommentID string `json:"comment_id,omitempty" jsonschema_description:"Comment ID, for update_comment."`
	Body      string `json:"body,omitempty" jsonschema_description:"Comment body, in Markdown. Required for create_comment and update_comment."`
}

// commentsTool implements gogent.Tool for Linear issue comments.
type commentsTool struct {
	*shared
	readonly bool
}

func newCommentsTool(s *shared, readonly bool) gogent.Tool {
	return &commentsTool{shared: s, readonly: readonly}
}

var (
	_ gogent.Tool       = (*commentsTool)(nil)
	_ gopi.ToolRenderer = (*commentsTool)(nil)
)

// Name returns the tool name.
func (t *commentsTool) Name() string { return "linear_comments" }

// Description returns the tool description for the model.
func (t *commentsTool) Description() string {
	return "Write Linear issue comments. create_comment adds a comment to an issue; " +
		"update_comment replaces a comment body. Both are writes and require user approval, " +
		"and the approval prompt shows the current body."
}

// Parameters returns the JSON Schema for the tool arguments.
func (t *commentsTool) Parameters() json.RawMessage {
	return parameters(new(commentsArgs), commentsOps, t.readonly)
}

// RequiresApproval returns whether the operation needs human approval. Both
// operations are writes. An update looks the current comment up so the prompt
// can show the old body.
func (t *commentsTool) RequiresApproval(ctx context.Context, raw json.RawMessage) (gogent.ApprovalDecision, error) {
	var args commentsArgs
	if err := parseArgs(raw, &args); err != nil {
		return gogent.ApprovalDecision{}, err
	}
	return approvalFor(commentsOps, t.readonly, args.Operation, func() string {
		return t.approvalReason(ctx, args)
	}), nil
}

// Execute runs the requested Linear operation.
func (t *commentsTool) Execute(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var args commentsArgs
	if err := parseArgs(raw, &args); err != nil {
		return nil, err
	}
	return t.execute(ctx, commentsOps, t.readonly, args.Operation,
		func(ctx context.Context, client linearClient, _ *resolver) (json.RawMessage, error) {
			return t.dispatch(ctx, client, args)
		})
}

// approvalReason names the change the user is approving.
func (t *commentsTool) approvalReason(ctx context.Context, args commentsArgs) string {
	switch args.Operation {
	case "update_comment":
		// Fetch the comment so RenderApproval can show the old body.
		t.lookupComment(ctx, args.CommentID)
		return fmt.Sprintf("Update comment %s", args.CommentID)
	case "create_comment":
		return fmt.Sprintf("Comment on issue %s", args.Issue)
	}
	return "Linear write operation: " + args.Operation
}

func (t *commentsTool) dispatch(ctx context.Context, client linearClient, args commentsArgs) (json.RawMessage, error) {
	switch args.Operation {
	case "create_comment":
		return t.createComment(ctx, client, args)
	case "update_comment":
		return t.updateComment(ctx, client, args)
	}
	return errorResult("invalid_operation", fmt.Sprintf("unknown operation %q", args.Operation))
}

func (t *commentsTool) createComment(ctx context.Context, client linearClient, args commentsArgs) (json.RawMessage, error) {
	if args.Issue == "" {
		return nil, invalid("issue is required for create_comment")
	}
	if args.Body == "" {
		return nil, invalid("body is required for create_comment")
	}
	issueID, err := t.issueID(ctx, client, args.Issue)
	if err != nil {
		return nil, err
	}
	comment, err := client.createComment(ctx, commentInput{IssueID: issueID, Body: args.Body})
	if err != nil {
		return nil, err
	}
	return operationResult("create_comment", map[string]any{"comment": comment})
}

func (t *commentsTool) updateComment(ctx context.Context, client linearClient, args commentsArgs) (json.RawMessage, error) {
	if args.CommentID == "" {
		return nil, invalid("comment_id is required for update_comment")
	}
	if args.Body == "" {
		return nil, invalid("body is required for update_comment")
	}
	comment, err := client.updateComment(ctx, args.CommentID, args.Body)
	if err != nil {
		return nil, err
	}
	t.forgetCurrentComment(args.CommentID)
	return operationResult("update_comment", map[string]any{"comment": comment})
}

// Headline names the operation and its subject on the tool line.
func (t *commentsTool) Headline(raw json.RawMessage) string {
	var args commentsArgs
	if json.Unmarshal(raw, &args) != nil {
		return ""
	}
	switch args.Operation {
	case "create_comment":
		return join("linear comment on", args.Issue)
	case "update_comment":
		return join("linear edit comment", args.CommentID)
	}
	return ""
}

// RenderApproval shows the comment a write will create or change.
func (t *commentsTool) RenderApproval(raw json.RawMessage) gopi.ToolView {
	var args commentsArgs
	if json.Unmarshal(raw, &args) != nil {
		return gopi.ToolView{}
	}
	var out approvalFields
	switch args.Operation {
	case "update_comment":
		out.add("Comment", args.CommentID)
		if comment, ok := t.currentComment(args.CommentID); ok {
			out.add("Body", change(truncate(comment.Body, 120), truncate(args.Body, 120)))
			return out.view()
		}
		out.add("Body", truncate(args.Body, 200))
	case "create_comment":
		out.add("Issue", args.Issue)
		out.add("Body", truncate(args.Body, 200))
	}
	return out.view()
}

// RenderResult draws a completed comments result.
func (t *commentsTool) RenderResult(_, raw json.RawMessage) gopi.ToolView {
	var result linearResult
	if json.Unmarshal(raw, &result) != nil {
		return gopi.ToolView{}
	}
	if result.Comment == nil {
		return gopi.ToolView{}
	}
	switch result.Operation {
	case "create_comment", "update_comment":
		verb := "Commented"
		if result.Operation == "update_comment" {
			verb = "Comment updated"
		}
		return confirmation(verb, "", truncate(result.Comment.Body, 120), result.Comment.URL)
	}
	return gopi.ToolView{}
}
