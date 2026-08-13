package client

import "context"

func (c *Client) ListDatabaseTypes(ctx context.Context) ([]DatabaseTypeInfo, error) {
	var resp DatabaseTypesResponse
	if err := c.do(ctx, "GET", "/databases/types", nil, &resp); err != nil {
		return nil, err
	}

	// Extract available sizes from config_schema for each database type.
	// The config_schema is an OpenAPI-style array of property definitions.
	// The "size" property has an "enum" field listing valid values.
	for i := range resp.Data {
		resp.Data[i].Sizes = extractSizesFromConfigSchema(resp.Data[i].ConfigSchema)
	}

	return resp.Data, nil
}

// extractSizesFromConfigSchema walks the config_schema array looking for
// a property named "size" that has an "enum" array of string values.
func extractSizesFromConfigSchema(schema []map[string]any) []string {
	for _, prop := range schema {
		name, _ := prop["name"].(string)
		if name != "size" {
			continue
		}
		enumVals, ok := prop["enum"].([]any)
		if !ok {
			continue
		}
		sizes := make([]string, 0, len(enumVals))
		for _, v := range enumVals {
			if s, ok := v.(string); ok {
				sizes = append(sizes, s)
			}
		}
		return sizes
	}
	return nil
}
