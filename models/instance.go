package models

import (
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"gitlab.com/george/shoya-go/config"
)

type WorldInstancePlayerCount struct {
	Total           int `json:"total"`
	PlatformWindows int `json:"platformWindows"`
	PlatformAndroid int `json:"platformAndroid"`
}

type WorldInstanceBlockedPlayers struct {
	ID    string `json:"id"`
	Until int64  `json:"until"`
}

type WorldInstance struct {
	ID              string                   `json:"id"`
	LastPing        int64                    `json:"lastPing"`
	InstanceID      string                   `json:"instanceId"` // entire string
	WorldID         string                   `json:"worldId"`
	InstanceType    string                   `json:"instanceType"` // privacy
	InstanceOwnerId string                   `json:"instanceOwnerId"`
	Capacity        int                      `json:"capacity"`
	OverCapacity    bool                     `json:"overCapacity"` // todo: investigate whether playercount.total can be used instead
	PlayerCount     WorldInstancePlayerCount `json:"playerCount"`
	Players         []string                 `json:"players"` // A list of players currently in this instance
	// PlayerTags     []string
	BlockedPlayers []WorldInstanceBlockedPlayers `json:"blockedPlayers"` // A list of players who are blocked from joining & until when
	ShortName      string                        `json:"shortName"`
	SecureName     string                        `json:"secureName"`
}

type InstanceJoinJWTClaims struct {
	JoinId              string   `json:"joinId"`
	UserId              string   `json:"userId"`
	Session             string   `json:"session"`
	IP                  string   `json:"ip"`
	Platform            Platform `json:"platform"`
	Location            string   `json:"location"`
	CanModerateInstance bool     `json:"canModerateInstance"`
	WorldAuthorId       string   `json:"worldAuthorId"`
	WorldCapacity       int      `json:"worldCapacity"`
	WorldName           string   `json:"worldName"`
	WorldTags           []string `json:"worldTags"`
	InstanceOwnerId     string   `json:"instanceOwnerId"`
	jwt.StandardClaims
}

func CreateJoinToken(u *User, w *World, ip string, location *Location) (string, error) {
	// TODO: Check whether location.IsStrict & check against presence service when made.
	joinId, _ := uuid.NewUUID()
	claims := InstanceJoinJWTClaims{
		JoinId:          "join_" + joinId.String(),
		UserId:          u.ID,
		Session:         "", // Unknown at the moment.
		IP:              ip,
		Location:        location.ID,
		WorldAuthorId:   w.AuthorID,
		WorldName:       w.Name,
		WorldTags:       w.Tags,
		WorldCapacity:   w.Capacity,
		InstanceOwnerId: location.OwnerID,
		StandardClaims: jwt.StandardClaims{
			Audience:  "VRChatNetworking",
			Issuer:    "VRChat",
			ExpiresAt: time.Now().Add(time.Hour * 1).Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.ApiConfiguration.JwtSecret.Get()))
}

func ValidateJoinToken(token string) (*InstanceJoinJWTClaims, error) {
	claims := InstanceJoinJWTClaims{}
	tkn, err := jwt.ParseWithClaims(token, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.ApiConfiguration.JwtSecret.Get()), nil
	})

	if err != nil {
		return nil, err
	}

	if !tkn.Valid {
		return nil, ErrInvalidJoinJWT
	}

	return &claims, nil
}
