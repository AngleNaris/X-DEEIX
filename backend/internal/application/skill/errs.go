package skill

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

var (
	// ErrSkillNotFound 表示技能不存在或当前用户无权访问。
	ErrSkillNotFound = apperr.New("skill.not_found", "skill not found")
	// ErrInvalidSkill 表示技能参数不合法。
	ErrInvalidSkill = apperr.New("request.invalid_skill", "invalid skill")
	// ErrSkillConflict 表示触发词在当前作用域内已存在。
<<<<<<< HEAD
	ErrSkillConflict = errors.New("skill trigger already exists")
	// ErrInvalidPackage 表示技能包不合法（zip 损坏、缺少 SKILL.md、越界等）。
	ErrInvalidPackage = errors.New("invalid skill package")
	// ErrPackageFileNotFound 表示包内文件不存在。
	ErrPackageFileNotFound = errors.New("skill package file not found")
	// ErrPackageFileUnreadable 表示包内文件为二进制，无法作为文本提供。
	ErrPackageFileUnreadable = errors.New("skill package file is not readable as text")
=======
	ErrSkillConflict = apperr.New("skill_trigger.already_exists", "skill trigger already exists")
>>>>>>> upstream/dev
)
