package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/oapi-codegen/nullable"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/avatar"
)

const defaultAvatarSize = 256

// toAvatar is what a profile shows; its picture URL changes with it.
func toAvatar(p accounts.Profile) api.Avatar {
	out := api.Avatar{Kind: api.AvatarKind(p.Avatar.Kind)}
	if p.Avatar.Kind == accounts.AvatarInitial {
		return out
	}
	url := "/avatars/" + p.ID + "?v=" + strconv.Itoa(p.Avatar.Version)
	out.Url = &url
	if p.Avatar.Kind == accounts.AvatarBuilder {
		if o, _, err := avatar.Parse([]byte(p.Avatar.Options)); err == nil {
			opts := toAvatarOptions(o)
			out.Options = &opts
		}
	}
	return out
}

func toAvatarOptions(o avatar.Options) api.AvatarOptions {
	opt := func(s *string) nullable.Nullable[string] {
		if s == nil {
			return nullable.NewNullNullable[string]()
		}
		return nullable.NewNullableWithValue(*s)
	}
	return api.AvatarOptions{Head: o.Head, Hair: o.Hair, Eyes: o.Eyes, Eyebrows: o.Eyebrows, Mouth: o.Mouth, Nose: o.Nose,
		Glasses: opt(o.Glasses), Beard: opt(o.Beard), Earrings: opt(o.Earrings), Freckles: opt(o.Freckles),
		HairColor: o.HairColor, SkinColor: o.SkinColor}
}

// parseAvatarOptions checks options as the builder sends them and returns
// them normalized.
func parseAvatarOptions(raw []byte) (avatar.Options, []byte, error) {
	o, norm, err := avatar.Parse(raw)
	if inv, ok := errors.AsType[*avatar.InvalidError](err); ok {
		field := "options"
		if inv.Field != "options" {
			field += "/" + inv.Field
		}
		return avatar.Options{}, nil, apierr.ValidationFailed(apierr.Detail{Field: field, Code: "invalid"})
	}
	return o, norm, err
}

// SetMyProfileAvatar shows the initial or a built avatar on a profile.
func (h *Handlers) SetMyProfileAvatar(ctx context.Context, req api.SetMyProfileAvatarRequestObject) (api.SetMyProfileAvatarResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	options := ""
	if b.Kind == api.AvatarUpdateKindBuilder {
		if b.Options == nil {
			return nil, apierr.ValidationFailed(apierr.Detail{Field: "options", Code: "required"})
		}
		raw, err := json.Marshal(b.Options)
		if err != nil {
			return nil, err
		}
		_, norm, err := parseAvatarOptions(raw)
		if err != nil {
			return nil, err
		}
		options = string(norm)
	}
	pr, err := h.accounts.SetAvatar(ctx, p, req.ProfileId, string(b.Kind), options)
	if err != nil {
		return nil, err
	}
	return api.SetMyProfileAvatar200JSONResponse(toProfile(pr)), nil
}

// SetMyProfilePhoto shows a photo on a profile.
func (h *Handlers) SetMyProfilePhoto(ctx context.Context, req api.SetMyProfilePhotoRequestObject) (api.SetMyProfilePhotoResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, apierr.BadRequest("missing body")
	}
	jpeg, err := avatar.NormalizePhoto(req.Body)
	if errors.Is(err, avatar.ErrBadPhoto) {
		return nil, apierr.ValidationFailed(apierr.Detail{Field: "photo", Code: "invalid"})
	}
	if err != nil {
		return nil, err
	}
	pr, err := h.accounts.SetPhoto(ctx, p, req.ProfileId, jpeg)
	if err != nil {
		return nil, err
	}
	return api.SetMyProfilePhoto200JSONResponse(toProfile(pr)), nil
}

// GetAvatar draws a profile's avatar, or serves its photo.
func (h *Handlers) GetAvatar(ctx context.Context, req api.GetAvatarRequestObject) (api.GetAvatarResponseObject, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	size := avatarSize(req.Params.Size)
	pic, err := h.accounts.Picture(ctx, req.ProfileId)
	if err != nil {
		return nil, err
	}
	cc, nosniff := imageCacheControl, "nosniff"
	headers := api.GetAvatar200ResponseHeaders{CacheControl: &cc, XContentTypeOptions: &nosniff}
	switch {
	case pic.Avatar.Kind == accounts.AvatarPhoto && pic.JPEG != nil:
		out := pic.JPEG
		if size != avatar.PhotoSize {
			if out, err = avatar.ResizeJPEG(pic.JPEG, size); err != nil {
				return nil, err
			}
		}
		return api.GetAvatar200ImagejpegResponse{Body: bytes.NewReader(out), ContentLength: int64(len(out)), Headers: headers}, nil
	case pic.Avatar.Kind == accounts.AvatarBuilder:
		o, _, err := avatar.Parse([]byte(pic.Avatar.Options))
		if err != nil {
			return nil, err
		}
		png, err := avatar.PNG(o, avatarColor(pic.Color), size)
		if err != nil {
			return nil, err
		}
		return api.GetAvatar200ImagepngResponse{Body: bytes.NewReader(png), ContentLength: int64(len(png)), Headers: headers}, nil
	}
	return nil, apierr.NotFound("this profile shows its initial")
}

// GetAvatarBuilder lists the builder's parts and colors.
func (h *Handlers) GetAvatarBuilder(ctx context.Context, _ api.GetAvatarBuilderRequestObject) (api.GetAvatarBuilderResponseObject, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	out := api.GetAvatarBuilder200JSONResponse{HairColors: avatar.HairColors, SkinColors: avatar.SkinColors,
		Defaults: toAvatarOptions(avatar.Defaults()), License: avatar.License}
	for _, p := range avatar.Parts() {
		out.Parts = append(out.Parts, api.AvatarPart{Name: api.AvatarPartName(p.Name), Optional: p.Optional, Variants: p.Variants})
	}
	return out, nil
}

// RenderAvatar draws options, for the builder's previews.
func (h *Handlers) RenderAvatar(ctx context.Context, req api.RenderAvatarRequestObject) (api.RenderAvatarResponseObject, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	o, _, err := parseAvatarOptions([]byte(req.Params.Options))
	if err != nil {
		return nil, err
	}
	bg := ""
	if c := req.Params.Color; c != nil {
		bg = avatarColor(*c)
	}
	png, err := avatar.PNG(o, bg, avatarSize(req.Params.Size))
	if err != nil {
		return nil, err
	}
	cc, nosniff := imageCacheControl, "nosniff"
	return api.RenderAvatar200ImagepngResponse{Body: bytes.NewReader(png), ContentLength: int64(len(png)),
		Headers: api.RenderAvatar200ResponseHeaders{CacheControl: &cc, XContentTypeOptions: &nosniff}}, nil
}

func avatarSize(s *api.AvatarSize) int {
	if s == nil || *s < 32 || *s > avatar.PhotoSize {
		return defaultAvatarSize
	}
	return *s
}

func avatarColor(i int) string {
	if i < 0 || i >= len(avatar.Colors) {
		return avatar.Colors[0]
	}
	return avatar.Colors[i]
}
