package spotify

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	pb "github.com/devgianlu/go-librespot/proto/spotify/playlist4"
	"google.golang.org/protobuf/proto"
)

// Playlist create/rename/delete go through spclient's playlist4 service
// (protobuf), as pathfinder has no mutations for them.

func (c *Client) username() (string, error) {
	uri := c.UserURI()
	if uri == "" {
		return "", errors.New("profile not loaded")
	}
	return strings.TrimPrefix(uri, "spotify:user:"), nil
}

func (c *Client) postProto(ctx context.Context, op, path string, msg proto.Message, out proto.Message) error {
	body, err := proto.Marshal(msg)
	if err != nil {
		return &Error{Op: op, Err: err}
	}
	resp, err := c.do(ctx, request{
		op: op, method: "POST", url: c.spclientURL + path, body: body,
		contentType: "application/x-protobuf", accept: "application/x-protobuf",
	})
	if err != nil || out == nil {
		return err
	}
	if err := proto.Unmarshal(resp, out); err != nil {
		return &Error{Op: op, Err: err}
	}
	return nil
}

func changeInfo(user string) *pb.ChangeInfo {
	return &pb.ChangeInfo{User: proto.String(user), Timestamp: proto.Int64(time.Now().UnixMilli())}
}

func renameOp(name string) *pb.Op {
	return &pb.Op{
		Kind: pb.Op_UPDATE_LIST_ATTRIBUTES.Enum(),
		UpdateListAttributes: &pb.UpdateListAttributes{
			NewAttributes: &pb.ListAttributesPartialState{Values: &pb.ListAttributes{Name: proto.String(name)}},
		},
	}
}

// CreatePlaylist creates a playlist and puts it at the top of the user's
// library. It returns the new playlist URI.
func (c *Client) CreatePlaylist(ctx context.Context, name string) (string, error) {
	user, err := c.username()
	if err != nil {
		return "", &Error{Op: "createPlaylist", Err: err}
	}
	var reply pb.CreateListReply
	err = c.postProto(ctx, "createPlaylist", "/playlist/v2/playlist",
		&pb.Delta{Ops: []*pb.Op{renameOp(name)}, Info: changeInfo(user)}, &reply)
	if err != nil {
		return "", err
	}
	uri := reply.GetUri()
	// The name in the create delta isn't applied; set it explicitly.
	if err := c.RenamePlaylist(ctx, uri, name, nil); err != nil {
		c.log.Warn("naming new playlist failed", "err", err)
	}
	err = c.rootlistChange(ctx, user, &pb.Op{
		Kind: pb.Op_ADD.Enum(),
		Add: &pb.Add{AddFirst: proto.Bool(true), Items: []*pb.Item{{
			Uri:        proto.String(uri),
			Attributes: &pb.ItemAttributes{Timestamp: proto.Int64(time.Now().UnixMilli())},
		}}},
	})
	c.InvalidateLibrary()
	return uri, err
}

// DeletePlaylist removes a playlist from the user's library (Spotify's
// "Delete" for own playlists is exactly this).
func (c *Client) DeletePlaylist(ctx context.Context, uri string) error {
	user, err := c.username()
	if err != nil {
		return &Error{Op: "deletePlaylist", Err: err}
	}
	err = c.rootlistChange(ctx, user, &pb.Op{
		Kind: pb.Op_REM.Enum(),
		Rem:  &pb.Rem{ItemsAsKey: proto.Bool(true), Items: []*pb.Item{{Uri: proto.String(uri)}}},
	})
	c.InvalidateLibrary()
	return err
}

func (c *Client) rootlistChange(ctx context.Context, user string, op *pb.Op) error {
	return c.postProto(ctx, "rootlist changes", "/playlist/v2/user/"+url.PathEscape(user)+"/rootlist/changes",
		&pb.ListChanges{Deltas: []*pb.Delta{{Ops: []*pb.Op{op}, Info: changeInfo(user)}}}, nil)
}

// RenamePlaylist changes a playlist's name and, if non-nil, its description.
func (c *Client) RenamePlaylist(ctx context.Context, uri, name string, description *string) error {
	user, err := c.username()
	if err != nil {
		return &Error{Op: "renamePlaylist", Err: err}
	}
	op := renameOp(name)
	if description != nil {
		op.UpdateListAttributes.NewAttributes.Values.Description = description
	}
	id := strings.TrimPrefix(uri, "spotify:playlist:")
	err = c.postProto(ctx, "renamePlaylist", "/playlist/v2/playlist/"+url.PathEscape(id)+"/changes",
		&pb.ListChanges{Deltas: []*pb.Delta{{Ops: []*pb.Op{op}, Info: changeInfo(user)}}}, nil)
	c.InvalidateLibrary()
	c.InvalidatePlaylists()
	return err
}
