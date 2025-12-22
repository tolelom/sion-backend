package models

import "time"

// ========================================
// AGV 모드 상수
// ========================================
const (
	ModeAuto   = "auto"   // 자동 모드 (랭덤 이동 + 타겟 발견 시 돌진)
	ModeManual = "manual" // 수동 모드 (웹에서 명령 수신)
)

// AGV 상태 상수
const (
	StateIdle      = "idle"      // 대기 중
	StateMoving    = "moving"    // 이동 중
	StateCharging  = "charging"  // 돌진 중 (사이온 궁극기 모드)
	StateSearching = "searching" // 타겟 탐색 중
	StateStopped   = "stopped"   // 정지 (에러/장애물)
	StateEmergency = "emergency" // 긴급 정지
)

// AGVMode - AGV 모드 타입
type AGVMode string

// AGVState - AGV 상태 타입
type AGVState string

// ========================================
// AGV 등록 정보
// ========================================
type AGVRegistration struct {
	AgentID   string          `json:"agent_id"`   // AGV ID
	Mode      AGVMode         `json:"mode"`       // 초기 모드
	Position  PositionData    `json:"position"`   // 초기 위치
	Timestamp int64           `json:"timestamp"`  // 등록 시간
}

// ========================================
// AGV 전체 상태
// ========================================
type AGVStatus struct {
	// 기본 정보
	ID         string    `json:"id"`          // AGV 고유 ID
	Name       string    `json:"name"`        // AGV 이름
	Connected  bool      `json:"connected"`   // 연결 상태
	LastUpdate time.Time `json:"last_update"` // 마지막 업데이트 시각

	// 위치 정보
	Position PositionData `json:"position"` // 현재 위치

	// 운영 상태
	Mode    AGVMode `json:"mode"`    // "auto" | "manual"
	State   AGVState `json:"state"`   // 현재 상태
	Speed   float64 `json:"speed"`   // 현재 속도 (m/s)
	Battery int     `json:"battery"` // 배터리 잔량 (0-100%)

	// 경로 정보
	CurrentPath *PathData     `json:"current_path"` // 현재 경로 (없으면 nil)
	TargetPos   *PositionData `json:"target_pos"`   // 목표 위치 (없으면 nil)

	// 타겟 정보
	TargetEnemy     *Enemy  `json:"target_enemy"`     // 현재 타겟 (없으면 nil)
	DetectedEnemies []Enemy `json:"detected_enemies"` // 감지된 모든 적

	// 센서 데이터
	Sensors SensorData `json:"sensors"` // 센서 정보
}

// ========================================
// 센서 데이터
// ========================================
type SensorData struct {
	// 거리 센서 (초음파/라이다)
	FrontDistance float64 `json:"front_distance"` // 전방 거리 (cm)
	LeftDistance  float64 `json:"left_distance"`  // 좌측 거리 (cm)
	RightDistance float64 `json:"right_distance"` // 우측 거리 (cm)

	// IMU/가속도 센서
	AccelX float64 `json:"accel_x"` // X축 가속도
	AccelY float64 `json:"accel_y"` // Y축 가속도
	AccelZ float64 `json:"accel_z"` // Z축 가속도
	GyroX  float64 `json:"gyro_x"`  // X축 회전
	GyroY  float64 `json:"gyro_y"`  // Y축 회전
	GyroZ  float64 `json:"gyro_z"`  // Z축 회전

	// 카메라 상태
	CameraActive    bool `json:"camera_active"`    // 카메라 활성화 여부
	ObjectsDetected int  `json:"objects_detected"` // 감지된 객체 수
}

// ========================================
// 모터 제어 데이터
// ========================================
type MotorControl struct {
	LeftSpeed  float64 `json:"left_speed"`  // 좌측 모터 속도 (-100 ~ 100)
	RightSpeed float64 `json:"right_speed"` // 우측 모터 속도 (-100 ~ 100)
	Duration   int     `json:"duration"`    // 제어 지속 시간 (ms)
}

// ========================================
// AGV 통계 데이터
// ========================================
type AGVStats struct {
	TotalDistance   float64   `json:"total_distance"`   // 총 이동 거리 (m)
	TotalTime       int64     `json:"total_time"`       // 총 운행 시간 (초)
	EnemiesDefeated int       `json:"enemies_defeated"` // 처치한 적 수
	Collisions      int       `json:"collisions"`       // 충돌 횟수
	StartTime       time.Time `json:"start_time"`       // 시작 시각
}
