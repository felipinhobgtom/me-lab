package http

import (
	"fmt"
	"testing"
)

func TestValidateStorageConfig(t *testing.T) {
	validCell := func() any {
		return map[string]any{
			"tier": "4k",
			"items": []any{
				map[string]any{"item_id": "minecraft:iron_ingot", "quantity": float64(64)},
			},
		}
	}

	tests := []struct {
		name    string
		blockID string
		config  map[string]any
		wantErr bool
	}{
		{
			name:    "drive accepts ten cells",
			blockID: "ae2:drive",
			config: map[string]any{
				"cells": []any{validCell(), validCell(), validCell(), validCell(), validCell(), validCell(), validCell(), validCell(), validCell(), validCell()},
			},
		},
		{
			name:    "drive rejects eleventh cell",
			blockID: "ae2:drive",
			config: map[string]any{
				"cells": []any{validCell(), validCell(), validCell(), validCell(), validCell(), validCell(), validCell(), validCell(), validCell(), validCell(), validCell()},
			},
			wantErr: true,
		},
		{
			name:    "cell rejects more than sixty three types",
			blockID: "ae2:drive",
			config: map[string]any{
				"cells": []any{map[string]any{
					"tier": "4k",
					"items": func() []any {
						items := make([]any, 64)
						for i := range items {
							items[i] = map[string]any{"item_id": fmt.Sprintf("test:item_%d", i), "quantity": float64(1)}
						}
						return items
					}(),
				}},
			},
			wantErr: true,
		},
		{
			name:    "cell rejects non-positive quantity",
			blockID: "ae2:drive",
			config: map[string]any{
				"cells": []any{map[string]any{
					"tier":  "4k",
					"items": []any{map[string]any{"item_id": "minecraft:iron_ingot", "quantity": float64(0)}},
				}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := validateStorageConfig(tt.blockID, tt.config)
			if (errMsg != "") != tt.wantErr {
				t.Fatalf("validateStorageConfig() error = %q, wantErr=%t", errMsg, tt.wantErr)
			}
		})
	}
}
