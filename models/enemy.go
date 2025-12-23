package models

// Enemy - 적 정보
type Enemy struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	HP       int          `json:"hp"`
	MaxHP    int          `json:"max_hp"`
	Position PositionData `json:"position"`
	Type     string       `json:"type"` // champion, minion, monster
	IsActive bool         `json:"is_active"`
}

// NewEnemy - 적 생성
func NewEnemy(id, name string, x, y float64) *Enemy {
	return &Enemy{
		ID:       id,
		Name:     name,
		HP:       100,
		MaxHP:    100,
		Position: PositionData{X: x, Y: y},
		Type:     "champion",
		IsActive: true,
	}
}

// IsDead - 사망 여부
func (e *Enemy) IsDead() bool {
	return e.HP <= 0
}

// TakeDamage - 데미지 받기
func (e *Enemy) TakeDamage(damage int) {
	e.HP -= damage
	if e.HP < 0 {
		e.HP = 0
	}
}
