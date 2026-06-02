export function castToType(key, value) {
  const definitions = {
    openAiTemp: {
      cast: (value) => Number(value),
    },
    openAiHistory: {
      cast: (value) => Number(value),
    },
    similarityThreshold: {
      cast: (value) => parseFloat(value),
    },
    topN: {
      cast: (value) => Number(value),
    },
    router_id: {
      cast: (value) => (value ? Number(value) : null),
    },
    compressEnabled: {
      cast: (value) => value, // "default", "true", or "false" — pass through as string
    },
    compressThreshold: {
      cast: (value) => (value === "" ? "" : value), // pass through string; empty = default
    },
    compressContextLen: {
      cast: (value) => (value === "" ? "" : value), // pass through string; empty = default
    },
  };

  if (!definitions.hasOwnProperty(key)) return value;
  return definitions[key].cast(value);
}
