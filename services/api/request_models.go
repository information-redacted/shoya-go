package api

import (
	"gitlab.com/george/shoya-go/config"
	"gitlab.com/george/shoya-go/models"
	"gitlab.com/george/shoya-go/models/service_types"
	"gorm.io/gorm"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// RegisterRequest is the model for requests sent to /auth/register.
type RegisterRequest struct {
	AcceptedTOSVersion int    `json:"acceptedTOSVersion"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	Email              string `json:"email"`
	Day                string `json:"day"`
	Month              string `json:"month"`
	Year               string `json:"year"`
	RecaptchaCode      string `json:"recaptchaCode"`
}

type ModerationRequest struct {
	CreatedAt   string                `json:"created"`
	ExpiresAt   string                `json:"expires"`
	Type        models.ModerationType `json:"type"`
	Reason      string                `json:"reason"`
	IsPermanent string                `json:"isPermanent"` // What the fuck, why???
	TargetID    string                `json:"targetUserId"`
	WorldID     string                `json:"worldId"`
	InstanceID  string                `json:"instanceId"`
}

type PlayerModerationRequest struct {
	Against string                      `json:"moderated"`
	Type    models.PlayerModerationType `json:"type"`
}

type UpdateUserRequest struct {
	AcceptedTOSVersion     int      `json:"acceptedTOSVersion"`
	Bio                    string   `json:"bio"`
	BioLinks               []string `json:"bioLinks"`
	Birthday               string   `json:"birthday"`
	CurrentPassword        string   `json:"currentPassword"`
	DisplayName            string   `json:"displayName"`
	Email                  string   `json:"email"`
	Password               string   `json:"password"`
	ProfilePictureOverride string   `json:"profilePicOverride"`
	Status                 string   `json:"status"`
	StatusDescription      string   `json:"statusDescription"`
	Tags                   []string `json:"tags"`
	Unsubscribe            bool     `json:"unsubscribe"`
	UserIcon               string   `json:"userIcon"`
	HomeLocation           string   `json:"homeLocation"`
}

var ValidLanguageTags = []string{"eng", "kor", "rus", "spa", "por", "zho", "deu", "jpn", "fra", "swe", "nld", "pol", "dan", "nor", "ita", "tha", "fin", "hun", "ces", "tur", "ara", "ron", "vie", "ukr", "ase", "bfi", "dse", "fsl", "kvk"}

func (r *UpdateUserRequest) EmailChecks(u *models.User) (bool, error) {
	if r.Email == "" {
		return false, nil
	}

	pwdMatch, err := u.CheckPassword(r.CurrentPassword)
	if !pwdMatch || err != nil {
		return false, models.ErrInvalidCredentialsInUserUpdate
	}

	if config.DB.Model(&models.User{}).Where("email = ?", r.Email).Or("pending_email = ?", r.Email).Error != gorm.ErrRecordNotFound {
		return false, models.ErrEmailAlreadyExistsInUserUpdate
	}

	u.PendingEmail = r.Email
	// TODO: Queue up verification email send
	return true, nil
}

func (r *UpdateUserRequest) PasswordChecks(u *models.User) (bool, error) {
	if r.Password == "" {
		return false, nil
	}

	pwdMatch, err := u.CheckPassword(r.CurrentPassword)
	if !pwdMatch || err != nil {
		return false, models.ErrInvalidCredentialsInUserUpdate
	}

	if len(r.Password) < 8 {
		return false, models.ErrPasswordTooSmall
	}

	err = u.ChangePassword(r.Password)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *UpdateUserRequest) BioLinksChecks(u *models.User) (bool, error) {
	if len(r.BioLinks) == 0 {
		return false, nil
	}

	if len(r.BioLinks) > 3 {
		return false, models.ErrInvalidBioLinks
	}

	for _, link := range r.BioLinks {
		if !strings.HasPrefix(link, "https://") && link != "" {
			return false, models.ErrInvalidBioLinks
		}
	}

	tempBioLinks := make([]string, 3)
	copy(tempBioLinks, u.BioLinks)

	changed := false

	if len(r.BioLinks) != len(u.BioLinks) {
		changed = true
	}

	for i, link := range r.BioLinks {
		if link != tempBioLinks[i] {
			changed = true
			break
		}
		changed = false
	}

	if !changed {
		return false, nil
	}

	for i, link := range r.BioLinks {
		if len(link) < 500 && !sliceContains(tempBioLinks, link) {
			// this behavior differs from the api.
			// vrchat normally responds with HTTP 400 {"error":{"message":"\"Link too long\"","status_code":400}}
			// but the website will actually render the bio link. *why*.

			tempBioLinks[i] = link
		}
	}

	u.BioLinks = tempBioLinks

	return changed, nil
}

func (r *UpdateUserRequest) StatusChecks(u *models.User) (bool, error) {
	var status service_types.UserStatus
	if r.Status == "" {
		return false, nil
	}

	switch strings.ToLower(r.Status) {
	case "join me":
		status = service_types.UserStatus(strings.ToLower(r.Status))
	case "active":
		status = service_types.UserStatus(strings.ToLower(r.Status))
	case "ask me":
		status = service_types.UserStatus(strings.ToLower(r.Status))
	case "busy":
		status = service_types.UserStatus(strings.ToLower(r.Status))
	case "offline":
		if !u.IsStaff() {
			return false, models.ErrInvalidStatusDescriptionInUserUpdate
		}
		status = service_types.UserStatus(strings.ToLower(r.Status))
	default:
		return false, models.ErrInvalidUserStatusInUserUpdate
	}

	u.Status = status
	return true, nil
}

func (r *UpdateUserRequest) StatusDescriptionChecks(u *models.User) (bool, error) {
	if r.StatusDescription == "" {
		return false, nil
	}

	if len(r.StatusDescription) > 32 {
		return false, models.ErrInvalidStatusDescriptionInUserUpdate
	}

	u.StatusDescription = r.StatusDescription
	return true, nil
}

func (r *UpdateUserRequest) BioChecks(u *models.User) (bool, error) {
	if r.Bio == "" {
		return false, nil
	}

	if len(r.Bio) > 512 {
		return false, models.ErrInvalidBioInUserUpdate
	}

	u.Bio = r.Bio
	return true, nil
}

func (r *UpdateUserRequest) UserIconChecks(u *models.User) (bool, error) {
	if r.UserIcon == "" {
		return false, nil
	}

	if !u.IsStaff() {
		return false, models.ErrSetUserIconWhenNotStaffInUserUpdate
	}

	u.UserIcon = r.UserIcon
	return true, nil
}

func (r *UpdateUserRequest) ProfilePicOverrideChecks(u *models.User) (bool, error) {
	if r.ProfilePictureOverride == "" {
		return false, nil
	}

	if !u.IsStaff() {
		return false, models.ErrSetProfilePicOverrideWhenNotStaffInUserUpdate
	}

	u.ProfilePicOverride = r.ProfilePictureOverride
	return true, nil
}

func (r *UpdateUserRequest) TagsChecks(u *models.User) (bool, error) {
	if len(r.Tags) == 0 {
		return false, nil
	}

	i := 0
	var tagsThatWillApply []string
	for _, tag := range r.Tags {
		if !strings.HasPrefix(tag, "language_") && !u.IsStaff() {
			continue
		} else if strings.HasPrefix(tag, "language_") {
			if !isValidLanguageTag(tag) {
				return false, models.ErrInvalidLanguageTagInUserUpdate
			}
			// Ensure that we do not add more that a total of 3 language tags to the user.
			if i++; i > 3 {
				return false, models.ErrTooManyLanguageTagsInUserUpdate
			}
		}

		tagsThatWillApply = append(tagsThatWillApply, tag)
	}

	for _, tag := range u.Tags {
		if strings.HasPrefix(tag, "system_") || strings.HasPrefix(tag, "admin_") {
			tagsThatWillApply = append(tagsThatWillApply, tag)
		}
	}

	u.Tags = tagsThatWillApply
	return true, nil
}

func (r *UpdateUserRequest) HomeLocationChecks(u *models.User) (bool, error) {
	if r.HomeLocation == "" {
		return false, nil
	}

	var w models.World
	tx := config.DB.Model(&models.World{}).Where("id = ?", r.HomeLocation).Find(&w)
	if tx.Error != nil {
		if tx.Error == gorm.ErrRecordNotFound {
			return false, models.ErrWorldNotFoundInUserUpdate
		}

		return false, nil
	}

	if w.ReleaseStatus == models.ReleaseStatusPrivate && (w.AuthorID != u.ID && !u.IsStaff()) {
		return false, models.ErrWorldPrivateNotOwnedByUserInUserUpdate
	}

	u.HomeWorldID = w.ID
	return true, nil
}

func isValidLanguageTag(tag string) bool {
	if !strings.HasPrefix(tag, "language_") {
		return false
	}

	split := strings.Split(tag, "_")
	if len(split) != 2 {
		return false
	}

	tagPart := split[1]
	for _, validTag := range ValidLanguageTags {
		if tagPart == validTag {
			return true
		}
	}

	return false
}

type AddTagsRequest struct {
	Tags []string `json:"tags"`
}

func (r *AddTagsRequest) TagsChecks(u *models.User) (bool, error) {
	if len(r.Tags) == 0 {
		return false, nil
	}

	i := 0
	var tagsThatWillApply []string
	for _, tag := range r.Tags {
		if !strings.HasPrefix(tag, "language_") && !u.IsStaff() {
			continue
		} else if strings.HasPrefix(tag, "language_") {
			if !isValidLanguageTag(tag) {
				return false, models.ErrInvalidLanguageTagInUserUpdate
			}
			// Ensure that we do not add more that a total of 3 language tags to the user.
			if i++; i > 3 {
				return false, models.ErrTooManyLanguageTagsInUserUpdate
			}
		}

		tagsThatWillApply = append(tagsThatWillApply, tag)
	}

	for _, tag := range u.Tags {
		if strings.HasPrefix(tag, "system_") || strings.HasPrefix(tag, "admin_") {
			tagsThatWillApply = append(tagsThatWillApply, tag)
		}
	}

	u.Tags = tagsThatWillApply
	return true, nil
}

type RemoveTagsRequest struct {
	Tags []string `json:"tags"`
}

func (r *RemoveTagsRequest) TagsChecks(u *models.User) (bool, error) {
	if len(r.Tags) == 0 {
		return false, nil
	}

	var tagsThatWillApply []string
	for _, tag := range r.Tags {
		if !strings.HasPrefix(tag, "language_") {
			continue
		}

		for _, uTag := range u.Tags {
			if tag != uTag {
				tagsThatWillApply = append(tagsThatWillApply, uTag)
			}
		}
	}

	u.Tags = tagsThatWillApply
	return true, nil
}

type CreateFileRequest struct {
	Name      string     `json:"name"`
	Extension string     `json:"extension"`
	MimeType  string     `json:"mimeType"`
	Versions  []struct{} `json:"versions"`
}

type CreateFileVersionRequest struct {
	FileMd5              string `json:"fileMd5"`
	FileSizeInBytes      int    `json:"fileSizeInBytes"`
	DeltaMd5             string `json:"deltaMd5"`
	DeltaSizeInBytes     int    `json:"deltaSizeInBytes"`
	SignatureMd5         string `json:"signatureMd5"`
	SignatureSizeInBytes int    `json:"signatureSizeInBytes"`
}

type CreateAvatarRequest struct {
	ID            string               `json:"id"`
	AssetUrl      string               `json:"assetUrl"`
	AssetVersion  string               `json:"assetVersion"`
	AuthorId      string               `json:"authorId"`
	AuthorName    string               `json:"authorName"`
	CreatedAt     string               `json:"created_at"`
	Description   string               `json:"description"`
	ImageUrl      string               `json:"imageUrl"`
	Name          string               `json:"name"`
	Platform      models.Platform      `json:"platform"`
	ReleaseStatus models.ReleaseStatus `json:"releaseStatus"`
	Tags          []string             `json:"tags"`
	TotalLikes    string               `json:"totalLikes"`
	TotalVisits   string               `json:"totalVisits"`
	UnityVersion  string               `json:"unityVersion"`
	UpdatedAt     string               `json:"updated_at"`
}

func (r *CreateAvatarRequest) HasValidUrls() bool {
	var apiUrl *url.URL
	var assetUrl *url.URL
	var imageUrl *url.URL
	var err error

	apiUrl, err = url.Parse(config.ApiConfiguration.ApiUrl.Get())
	if err != nil {
		return false
	}

	if r.AssetUrl != "" {
		assetUrl, err = url.Parse(r.AssetUrl)
		if err != nil {
			return false
		}
	}

	if r.ImageUrl != "" {
		imageUrl, err = url.Parse(r.AssetUrl)
		if err != nil {
			return false
		}
	}

	if r.AssetUrl != "" {
		if apiUrl.Host != assetUrl.Host {
			return false
		}
	}

	if r.ImageUrl != "" {
		if apiUrl.Host != imageUrl.Host {
			return false
		}
	}

	if r.AssetUrl != "" {
		if !strings.HasPrefix(assetUrl.Path, "/api/1/file/") {
			return false
		}
	}

	if r.ImageUrl != "" {
		if !strings.HasPrefix(imageUrl.Path, "/api/1/file/") {
			return false
		}
	}

	return true
}

func (r *CreateAvatarRequest) ParseTags() []string {
	var tags = []string{}
	for _, tag := range r.Tags {
		if strings.HasPrefix(tag, "author_tag_") {
			tags = append(tags, tag)
		}
	}

	return tags
}

func (r *CreateAvatarRequest) GetFileID() (string, error) {
	re := regexp.MustCompile(`(?i)\/api\/1\/file\/(file_[\dA-F]{8}-[\dA-F]{4}-4[\dA-F]{3}-[89AB][\dA-F]{3}-[\dA-F]{12})`)
	val := re.FindStringSubmatch(r.AssetUrl)
	if val == nil {
		return "", models.ErrUrlParseFailed
	}
	return val[1], nil
}

func (r *CreateAvatarRequest) GetFileVersion() (int, error) {
	re := regexp.MustCompile(`(?i)\/api\/1\/file\/file_[\dA-F]{8}-[\dA-F]{4}-4[\dA-F]{3}-[89AB][\dA-F]{3}-[\dA-F]{12}/(\d*)/`)
	val := re.FindStringSubmatch(r.AssetUrl)
	if val == nil {
		return 0, models.ErrUrlParseFailed
	}

	v, err := strconv.Atoi(val[1])
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (r *CreateAvatarRequest) GetImageID() (string, error) {
	re := regexp.MustCompile(`(?i)\/api\/1\/file\/(file_[\dA-F]{8}-[\dA-F]{4}-4[\dA-F]{3}-[89AB][\dA-F]{3}-[\dA-F]{12})`)
	val := re.FindStringSubmatch(r.ImageUrl)
	if val == nil {
		return "", models.ErrUrlParseFailed
	}
	return val[1], nil
}

type CreateWorldRequest struct {
	ID            string               `json:"id"`
	AssetUrl      string               `json:"assetUrl"`
	AssetVersion  int                  `json:"assetVersion"`
	AuthorId      string               `json:"authorId"`
	AuthorName    string               `json:"authorName"`
	Capacity      int                  `json:"capacity"`
	CreatedAt     string               `json:"created_at"`
	Description   string               `json:"description"`
	ImageUrl      string               `json:"imageUrl"`
	Name          string               `json:"name"`
	Platform      models.Platform      `json:"platform"`
	ReleaseStatus models.ReleaseStatus `json:"releaseStatus"`
	Tags          []string             `json:"tags"`
	TotalLikes    string               `json:"totalLikes"`
	TotalVisits   string               `json:"totalVisits"`
	UnityVersion  string               `json:"unityVersion"`
	UpdatedAt     string               `json:"updated_at"`
}

func (r *CreateWorldRequest) HasValidUrls() bool {
	var apiUrl *url.URL
	var assetUrl *url.URL
	var imageUrl *url.URL
	var err error

	apiUrl, err = url.Parse(config.ApiConfiguration.ApiUrl.Get())
	if err != nil {
		return false
	}

	if r.AssetUrl != "" {
		assetUrl, err = url.Parse(r.AssetUrl)
		if err != nil {
			return false
		}
	}

	if r.ImageUrl != "" {
		imageUrl, err = url.Parse(r.ImageUrl)
		if err != nil {
			return false
		}
	}

	if r.AssetUrl != "" {
		if apiUrl.Host != assetUrl.Host {
			return false
		}
	}

	if r.ImageUrl != "" {
		if apiUrl.Host != imageUrl.Host {
			return false
		}
	}

	if r.AssetUrl != "" {
		if !strings.HasPrefix(assetUrl.Path, "/api/1/file/") {
			return false
		}
	}

	if r.ImageUrl != "" {
		if !strings.HasPrefix(imageUrl.Path, "/api/1/file/") {
			return false
		}
	}

	return true
}

func (r *CreateWorldRequest) ParseTags() []string {
	var tags = []string{}
	for _, tag := range r.Tags {
		if strings.HasPrefix(tag, "author_tag_") || tag == "debug_allowed" {
			tags = append(tags, tag)
		}
	}

	return tags
}

func (r *CreateWorldRequest) GetFileID() (string, error) {
	re := regexp.MustCompile(`(?i)\/api\/1\/file\/(file_[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12})`)
	val := re.FindStringSubmatch(r.AssetUrl)
	if val == nil {
		return "", models.ErrUrlParseFailed
	}
	return val[1], nil
}

func (r *CreateWorldRequest) GetFileVersion() (int, error) {
	re := regexp.MustCompile(`(?i)\/api\/1\/file\/file_[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}/(\d*)/`)
	val := re.FindStringSubmatch(r.AssetUrl)
	if val == nil {
		return 0, models.ErrUrlParseFailed
	}

	v, err := strconv.Atoi(val[1])
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (r *CreateWorldRequest) GetImageID() (string, error) {
	re := regexp.MustCompile(`(?i)\/api\/1\/file\/(file_[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12})`)
	val := re.FindStringSubmatch(r.ImageUrl)
	if val == nil {
		return "", models.ErrUrlParseFailed
	}
	return val[1], nil
}

type PutVisitsRequest struct {
	UserId  string `json:"userId,omitempty"`
	WorldId string `json:"worldId"`
}

type PutJoinsRequest struct {
	UserId  string `json:"userId,omitempty"`
	WorldId string `json:"worldId"`
}
