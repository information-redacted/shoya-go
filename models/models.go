package models

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel is the model used by all Gorm models.
type BaseModel struct {
	ID        string         `gorm:"primarykey;unique" json:"id"`
	CreatedAt int64          `json:"-"`
	UpdatedAt int64          `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Platform is the supported platform for an asset.
// PlatformWindows ("standalonewindows"): The asset has a build for Windows versions of the game.
// PlatformAndroid ("android"): The asset has a build for Android (Quest) versions of the game.
type Platform string

const (
	PlatformWindows Platform = "standalonewindows"
	PlatformAndroid Platform = "android"
)

// ReleaseStatus is the state an asset is released in.
// ReleaseStatusPublic ("public"):
//    For worlds, this means that anyone can search for it, create instances, and favorite it.
//    For avatars, this means that anyone can use it.
// ReleaseStatusPrivate ("private"):
//    For worlds, this means that only people who have the ID can create instances. It cannot be favorited.
//    For avatars, this means that only the avatar author can use it.
// ReleaseStatusHidden ("hidden"): This is the status that deleted content goes into.
type ReleaseStatus string

const (
	ReleaseStatusPublic  ReleaseStatus = "public"
	ReleaseStatusPrivate ReleaseStatus = "private"
	ReleaseStatusHidden  ReleaseStatus = "hidden"
)

type WorldUnityPackage struct {
	BaseModel
	BelongsToAssetID string
	FileID           string
	File             File     `json:"-"`
	FileVersion      int      `json:"-"`
	Version          int      `json:"assetVersion"`
	Platform         Platform `json:"platform"`
	UnityVersion     string   `json:"unityVersion"`
	UnitySortNumber  int      `json:"unitySortNumber"`
}

func (u *WorldUnityPackage) BeforeCreate(*gorm.DB) (err error) {
	u.ID = "unp_" + uuid.New().String()
	return
}

func (u *WorldUnityPackage) GetAPIUnityPackage(withAssetUrl bool) *APIUnityPackage {
	var assetUrl = ""
	if withAssetUrl {
		assetUrl = u.File.GetLatestVersion().GetFileUrl()
	}
	return &APIUnityPackage{
		ID:              u.ID,
		CreatedAt:       time.Unix(u.CreatedAt, 0).UTC().Format(time.RFC3339Nano),
		AssetUrl:        assetUrl,
		AssetUrlObject:  fiber.Map{},
		Platform:        u.Platform,
		UnityVersion:    u.UnityVersion,
		UnitySortNumber: u.UnitySortNumber,
		PluginUrlObject: fiber.Map{},
	}
}

type AvatarUnityPackage struct {
	BaseModel
	BelongsToAssetID string
	FileID           string   `json:"-"`
	File             File     `json:"-"`
	FileVersion      int      `json:"-"`
	Version          int      `json:"assetVersion"`
	Platform         Platform `json:"platform"`
	UnityVersion     string   `json:"unityVersion"`
	UnitySortNumber  int      `json:"unitySortNumber"`
}

func (u *AvatarUnityPackage) BeforeCreate(*gorm.DB) (err error) {
	u.ID = "unp_" + uuid.New().String()
	return
}

func (u *AvatarUnityPackage) GetAPIUnityPackage() *APIUnityPackage {
	return &APIUnityPackage{
		ID:              u.ID,
		CreatedAt:       time.Unix(u.CreatedAt, 0).UTC().Format(time.RFC3339Nano),
		AssetUrl:        u.File.GetVersion(u.FileVersion).GetFileUrl(),
		AssetVersion:    u.Version,
		Platform:        u.Platform,
		UnityVersion:    u.UnityVersion,
		UnitySortNumber: u.UnitySortNumber,
	}
}

type APIUnityPackage struct {
	ID              string      `json:"id"`
	AssetUrl        string      `json:"assetUrl"`
	AssetUrlObject  interface{} `json:"assetUrlObject"`
	AssetVersion    int         `json:"assetVersion"`
	CreatedAt       string      `json:"created_at"`
	Platform        Platform    `json:"platform"`
	PluginUrl       string      `json:"pluginUrl"`
	PluginUrlObject interface{} `json:"pluginUrlObject"`
	UnityVersion    string      `json:"unityVersion"`
	UnitySortNumber int         `json:"unitySortNumber"`
}

var InstanceFlagRegex = regexp.MustCompile(`^(?P<flagName>.*)?\((?P<flagValue>.*)\)`)

type InstanceFlagType string

var (
	InstanceFlagTypeNone         InstanceFlagType = "none"
	InstanceFlagTypePrivacy      InstanceFlagType = "privacy"
	InstanceFlagTypeRegion       InstanceFlagType = "region"
	InstanceFlagTypeNonce        InstanceFlagType = "nonce"
	InstanceFlagTypeCanReqInvite InstanceFlagType = "canRequestInvite"
	InstanceFlagTypeStrict       InstanceFlagType = "strict"
)

type InstanceType string

var (
	InstanceTypePublic  InstanceType = "public"
	InstanceTypeHidden  InstanceType = "hidden"
	InstanceTypePrivate InstanceType = "private"
)

var AllowedInstanceTypes = []string{"hidden", "friends", "private"}
var AllowedInstanceRegions = []string{"us", "usw", "use", "eu", "jp"}

type Location struct {
	ID               string `json:"id"`
	WorldID          string `json:"worldId"`          // WorldID is the id of the world the instance is for.
	InstanceID       string `json:"instanceId"`       // InstanceID is the instance's identifier (usually 5 numbers).
	LocationString   string `json:"locationString"`   // LocationString is the string minus the world id prefix.
	InstanceType     string `json:"instanceType"`     // InstanceType is the instance's privacy setting. | Valid settings: public, hidden, friends, private.
	OwnerID          string `json:"ownerId"`          // OwnerID is the id of the instance's creator.
	Nonce            string `json:"nonce"`            // Nonce is a "shared key" for use in non-public instances.
	Region           string `json:"region"`           // Region is the Photon region (us, or blank == usw photon) | Valid regions: us, use, eu, jp.
	CanRequestInvite bool   `json:"canRequestInvite"` // CanRequestInvite turns an instance of InstanceType: private (invite-only) to an invite+.
	IsStrict         bool   `json:"strict"`           // IsStrict ensures that the instance is only joinable if the user is friends with the creator.
}

// ParseLocationString parses the location string provided in a request.
func ParseLocationString(s string) (*Location, error) {
	var location = Location{}
	s1 := strings.Split(s, ":")
	if len(s1) < 2 {
		return nil, fmt.Errorf("invalid instance id: %s", s1)
	}

	location.ID = s
	location.WorldID = s1[0]        // wrld_{uuid}
	location.LocationString = s1[1] // 00000~xxx

	s2 := strings.Split(s1[1], "~")
	location.InstanceID = s2[0]
	location.InstanceType = "public" // Default to public
	location.Region = "usw"          // Default to usw

	err := parseInstanceFlags(s2[1:], &location)
	if err != nil {
		return nil, err
	}

	if location.InstanceType == "public" && location.IsStrict {
		return nil, errors.New("can not tag a public instance as strict")
	}

	return &location, nil
}

func parseInstanceFlags(flags []string, l *Location) error {
	for _, flag := range flags {
		flagType, flagName, flagValue, err := parseInstanceFlag(flag)
		if err != nil {
			return err
		}
		switch flagType {
		case InstanceFlagTypePrivacy:
			for _, allowedInstanceType := range AllowedInstanceTypes {
				if allowedInstanceType == flagName {
					l.InstanceType = flagName
					l.OwnerID = flagValue.(string)
					break
				}
			}

		case InstanceFlagTypeRegion:
			instanceRegion := flagValue.(string)
			for _, allowedInstanceRegion := range AllowedInstanceRegions {
				if allowedInstanceRegion == instanceRegion {
					l.Region = instanceRegion
					break
				}
			}

		case InstanceFlagTypeNonce:
			l.Nonce = flagValue.(string)
		case InstanceFlagTypeCanReqInvite:
			l.CanRequestInvite = true // Will always return true if this flag exists
		case InstanceFlagTypeStrict:
			l.IsStrict = true // ^^
		}
	}
	return nil
}

func parseInstanceFlag(flag string) (InstanceFlagType, string, interface{}, error) {
	if strings.ToLower(flag) == "canrequestinvite" {
		return InstanceFlagTypeCanReqInvite, "canrequestinvite", true, nil
	}

	if strings.ToLower(flag) == "strict" {
		return InstanceFlagTypeStrict, "strict", true, nil
	}

	m := InstanceFlagRegex.FindStringSubmatch(flag)
	if len(m) != 3 {
		return InstanceFlagTypeNone, "", "", fmt.Errorf("could not find 3 matches for match \"%s\" in parseinstanceflag", m[0])
	}

	var flagType = InstanceFlagType(m[1])
	for _, instanceType := range AllowedInstanceTypes {
		if m[1] == instanceType {
			flagType = InstanceFlagTypePrivacy
		}
	}

	// TODO: stricter checks here

	return flagType, m[1], m[2], nil
}
