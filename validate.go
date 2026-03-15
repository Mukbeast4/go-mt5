package gomt5

import "fmt"

func (r *TradeRequest) Validate() error {
	switch r.Action {
	case TradeActionDeal:
		if r.Symbol == "" {
			return fmt.Errorf("symbol required for deal action")
		}
		if r.Volume <= 0 {
			return fmt.Errorf("volume must be positive for deal action")
		}
		if r.Price < 0 {
			return fmt.Errorf("price must be non-negative for deal action")
		}
	case TradeActionPending:
		if r.Symbol == "" {
			return fmt.Errorf("symbol required for pending action")
		}
		if r.Volume <= 0 {
			return fmt.Errorf("volume must be positive for pending action")
		}
		if r.Price <= 0 {
			return fmt.Errorf("price must be positive for pending action")
		}
	case TradeActionSLTP:
		if r.Position == 0 {
			return fmt.Errorf("position required for SLTP action")
		}
		if r.SL == 0 && r.TP == 0 {
			return fmt.Errorf("at least one of SL or TP required for SLTP action")
		}
	case TradeActionModify:
		if r.Order == 0 {
			return fmt.Errorf("order required for modify action")
		}
	case TradeActionRemove:
		if r.Order == 0 {
			return fmt.Errorf("order required for remove action")
		}
	case TradeActionCloseBy:
		if r.Position == 0 {
			return fmt.Errorf("position required for close-by action")
		}
		if r.PositionBy == 0 {
			return fmt.Errorf("position_by required for close-by action")
		}
	default:
		return fmt.Errorf("unknown action %d", r.Action)
	}
	return nil
}
