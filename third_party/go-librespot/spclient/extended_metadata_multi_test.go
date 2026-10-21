//go:build test_unit

package spclient_test

import (
	"net/http"

	librespot "github.com/devgianlu/go-librespot"
	extmetadatapb "github.com/devgianlu/go-librespot/proto/spotify/extendedmetadata"
	audiofilespb "github.com/devgianlu/go-librespot/proto/spotify/extendedmetadata/audiofiles"
	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// One request carries both extensions, and each answer lands in its message.
func (suite *RequestSuite) TestExtendedMetadataMulti() {
	id, err := librespot.SpotifyIdFromUri("spotify:track:574y1r7o2tRA009FW0LE7v")
	suite.Require().NoError(err)
	entry := func(kind extmetadatapb.ExtensionKind, msg proto.Message) *extmetadatapb.EntityExtensionDataArray {
		data, err := anypb.New(msg)
		suite.Require().NoError(err)
		return &extmetadatapb.EntityExtensionDataArray{ExtensionKind: kind, ExtensionData: []*extmetadatapb.EntityExtensionData{{
			Header: &extmetadatapb.EntityExtensionDataHeader{StatusCode: 200}, EntityUri: id.Uri(), ExtensionData: data,
		}}}
	}
	body, err := proto.Marshal(&extmetadatapb.BatchedExtensionResponse{ExtendedMetadata: []*extmetadatapb.EntityExtensionDataArray{
		entry(extmetadatapb.ExtensionKind_TRACK_V4, &metadatapb.Track{Name: proto.String("Speak to Me")}),
		entry(extmetadatapb.ExtensionKind_AUDIO_FILES, &audiofilespb.AudioFilesExtensionResponse{Files: []*audiofilespb.ExtendedAudioFile{{}}}),
	}})
	suite.Require().NoError(err)
	suite.handler = func(_ int, w http.ResponseWriter) { _, _ = w.Write(body) }

	var track metadatapb.Track
	var files audiofilespb.AudioFilesExtensionResponse
	err = suite.spclient.ExtendedMetadataMulti(suite.T().Context(), *id, map[extmetadatapb.ExtensionKind]proto.Message{
		extmetadatapb.ExtensionKind_TRACK_V4:    &track,
		extmetadatapb.ExtensionKind_AUDIO_FILES: &files,
	})
	suite.Require().NoError(err)
	suite.Equal("Speak to Me", track.GetName())
	suite.Len(files.Files, 1)

	var req extmetadatapb.BatchedEntityRequest
	suite.Require().NoError(proto.Unmarshal(suite.requests()[0].body, &req))
	suite.Require().Len(req.EntityRequest, 1)
	suite.Len(req.EntityRequest[0].Query, 2)
}

// A missing extension is an error, so the caller falls back to asking apart.
func (suite *RequestSuite) TestExtendedMetadataMultiMissingKind() {
	id, err := librespot.SpotifyIdFromUri("spotify:track:574y1r7o2tRA009FW0LE7v")
	suite.Require().NoError(err)
	body, _ := proto.Marshal(&extmetadatapb.BatchedExtensionResponse{})
	suite.handler = func(_ int, w http.ResponseWriter) { _, _ = w.Write(body) }

	var track metadatapb.Track
	err = suite.spclient.ExtendedMetadataMulti(suite.T().Context(), *id, map[extmetadatapb.ExtensionKind]proto.Message{
		extmetadatapb.ExtensionKind_TRACK_V4: &track,
	})
	suite.Error(err)
}
