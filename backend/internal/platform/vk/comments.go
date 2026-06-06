package vk

import (
	"context"
	"fmt"

	vksdk "github.com/SevereCloud/vksdk/v3/api"
)

// ReplyToWallComment posts a reply in the same thread as the comment.
// ownerID is the wall owner (negative for communities, positive for users).
// For community walls owned by THIS community, ownerID = -groupID.
func (c *Client) ReplyToWallComment(ctx context.Context, ownerID, postID, replyToCommentID int, text string) (int, error) {
	if err := c.CheckRateLimit(ctx); err != nil {
		return 0, err
	}
	id, err := c.VK.WallCreateComment(vksdk.Params{
		"owner_id":         ownerID,
		"post_id":          postID,
		"reply_to_comment": replyToCommentID,
		"message":          text,
		"from_group":       1,
	})
	if err != nil {
		return 0, fmt.Errorf("vk.ReplyToWallComment: %w", err)
	}
	return id.CommentID, nil
}
