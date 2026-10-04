package domain

import "time"

type VesselState string

const VesselReservationWindow = 30 * time.Minute

const (
	VesselAvailable VesselState = "available"
	VesselReserved  VesselState = "reserved"
	VesselInUse     VesselState = "in_use"
	VesselCleaning  VesselState = "cleaning"
	VesselRetired   VesselState = "retired"
)

type Vessel struct {
	ID                    string      `json:"id"`
	Code                  string      `json:"code"`
	CapacityL             float64     `json:"capacity_liters"`
	State                 VesselState `json:"state"`
	ActiveRunID           *string     `json:"active_run_id,omitempty"`
	ReservationGeneration int64       `json:"reservation_generation,omitempty"`
	ReservationExpiresAt  *time.Time  `json:"reservation_expires_at,omitempty"`
	Version               int64       `json:"version"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
}

func (v Vessel) Validate() error {
	if err := ValidateVesselCode(v.Code, "code"); err != nil {
		return err
	}
	if v.CapacityL < 10 || v.CapacityL > 5000 {
		return FieldError(CodeInvalidInput, "capacity_liters",
			"must be between 10 and 5000 liters")
	}
	return nil
}

func (v Vessel) ReservationExpired(now time.Time) bool {
	return v.State == VesselReserved &&
		v.ReservationExpiresAt != nil &&
		!v.ReservationExpiresAt.After(now.UTC())
}

func (v *Vessel) Reserve(runID string, expiresAt, now time.Time) error {
	available := v.State == VesselAvailable && v.ActiveRunID == nil
	reclaimable := v.ReservationExpired(now)
	if !available && !reclaimable {
		return NewError(CodeVesselUnavailable, "vessel is not available for reservation")
	}
	run := runID
	v.State = VesselReserved
	v.ActiveRunID = &run
	v.ReservationGeneration++
	v.ReservationExpiresAt = reservationTime(expiresAt)
	v.UpdatedAt = now.UTC()
	v.Version++
	return nil
}

func (v *Vessel) StartRun(runID string, generation int64, now time.Time) error {
	if v.State != VesselReserved {
		return NewError(CodeVesselState, "vessel is not reserved by this run")
	}
	if v.ActiveRunID == nil || *v.ActiveRunID != runID {
		return NewError(CodeVesselState, "vessel is reserved by a different run")
	}
	if v.ReservationGeneration != generation {
		return NewError(CodeVesselState, "vessel reservation is no longer current")
	}
	if v.ReservationExpiresAt == nil || !v.ReservationExpiresAt.After(now.UTC()) {
		return NewError(CodeVesselState, "vessel reservation has expired")
	}
	run := runID
	v.State = VesselInUse
	v.ActiveRunID = &run
	v.ReservationGeneration = generation
	v.ReservationExpiresAt = nil
	v.UpdatedAt = now.UTC()
	v.Version++
	return nil
}

func (v *Vessel) StartCleaning(runID string, now time.Time) error {
	if (v.State != VesselInUse && v.State != VesselReserved) ||
		v.ActiveRunID == nil || *v.ActiveRunID != runID {
		return NewError(CodeVesselState, "vessel cannot enter cleaning for this run")
	}
	v.State = VesselCleaning
	v.ReservationExpiresAt = nil
	v.UpdatedAt = now.UTC()
	v.Version++
	return nil
}

func (v *Vessel) FinishCleaning(now time.Time) error {
	if v.State != VesselCleaning || v.ActiveRunID == nil {
		return NewError(CodeVesselState, "vessel is not waiting for cleaning")
	}
	v.State = VesselAvailable
	v.ActiveRunID = nil
	v.ReservationGeneration++
	v.ReservationExpiresAt = nil
	v.UpdatedAt = now.UTC()
	v.Version++
	return nil
}

func (v *Vessel) ReleaseReservation(runID string, now time.Time) error {
	if v.State != VesselReserved && v.State != VesselInUse {
		return NewError(CodeVesselState, "vessel does not have a releasable run")
	}
	if v.ActiveRunID == nil || *v.ActiveRunID != runID {
		return NewError(CodeVesselState, "vessel belongs to a different run")
	}
	v.State = VesselAvailable
	v.ActiveRunID = nil
	v.ReservationGeneration++
	v.ReservationExpiresAt = nil
	v.UpdatedAt = now.UTC()
	v.Version++
	return nil
}

func (v *Vessel) Retire(now time.Time) error {
	if v.State == VesselRetired {
		return NewError(CodeVesselState, "vessel is already retired")
	}
	if v.State != VesselAvailable || v.ActiveRunID != nil {
		return NewError(CodeVesselState, "only an available vessel can be retired")
	}
	v.State = VesselRetired
	v.ReservationExpiresAt = nil
	v.UpdatedAt = now.UTC()
	v.Version++
	return nil
}

func reservationTime(value time.Time) *time.Time {
	stamp := value.UTC()
	return &stamp
}
