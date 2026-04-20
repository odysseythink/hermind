import { z } from 'zod';

// FieldKind strings produced by gateway/platforms.FieldKind.String().
export const FieldKindSchema = z.enum([
  'unknown', 'string', 'int', 'bool', 'secret', 'enum',
]);
export type FieldKind = z.infer<typeof FieldKindSchema>;

export const SchemaFieldSchema = z.object({
  name: z.string(),
  label: z.string(),
  help: z.string().optional(),
  kind: FieldKindSchema,
  required: z.boolean().optional(),
  default: z.unknown().optional(),
  enum: z.array(z.string()).optional(),
});
export type SchemaField = z.infer<typeof SchemaFieldSchema>;

export const SchemaDescriptorSchema = z.object({
  type: z.string(),
  display_name: z.string(),
  summary: z.string().optional(),
  fields: z.array(SchemaFieldSchema),
});
export type SchemaDescriptor = z.infer<typeof SchemaDescriptorSchema>;

export const PlatformsSchemaResponseSchema = z.object({
  descriptors: z.array(SchemaDescriptorSchema),
});
export type PlatformsSchemaResponse = z.infer<typeof PlatformsSchemaResponseSchema>;

// Config is shaped like the backend Config struct, but we only model
// the gateway.platforms subtree explicitly — everything else is kept
// as-is in the draft object so PUT round-trips the unknown fields.
export const PlatformInstanceSchema = z.object({
  enabled: z.boolean().optional(),
  type: z.string(),
  options: z.record(z.string(), z.string()).optional(),
});
export type PlatformInstance = z.infer<typeof PlatformInstanceSchema>;

export const ConfigSchema = z.object({
  gateway: z.object({
    platforms: z.record(z.string(), PlatformInstanceSchema).optional(),
  }).optional(),
}).catchall(z.unknown());
export type Config = z.infer<typeof ConfigSchema>;

export const ConfigResponseSchema = z.object({ config: ConfigSchema });

export const ApplyResultSchema = z.object({
  ok: z.boolean(),
  restarted: z.array(z.string()).optional(),
  errors: z.record(z.string(), z.string()).optional(),
  took_ms: z.number(),
  error: z.string().optional(),
});
export type ApplyResult = z.infer<typeof ApplyResultSchema>;

export const PlatformTestResponseSchema = z.object({
  ok: z.boolean(),
  error: z.string().optional(),
});
export type PlatformTestResponse = z.infer<typeof PlatformTestResponseSchema>;

export const RevealResponseSchema = z.object({
  value: z.string(),
});
export type RevealResponse = z.infer<typeof RevealResponseSchema>;

// Config section kinds produced by descriptor.FieldKind.String(). Adds
// 'float' on top of the platform FieldKind set.
export const ConfigFieldKindSchema = z.enum([
  'string', 'int', 'bool', 'secret', 'enum', 'float',
]);
export type ConfigFieldKind = z.infer<typeof ConfigFieldKindSchema>;

export const ConfigPredicateSchema = z.object({
  field: z.string(),
  equals: z.unknown(),
});
export type ConfigPredicate = z.infer<typeof ConfigPredicateSchema>;

export const ConfigFieldSchema = z.object({
  name: z.string(),
  label: z.string(),
  help: z.string().optional(),
  kind: ConfigFieldKindSchema,
  required: z.boolean().optional(),
  default: z.unknown().optional(),
  enum: z.array(z.string()).optional(),
  visible_when: ConfigPredicateSchema.optional(),
});
export type ConfigField = z.infer<typeof ConfigFieldSchema>;

export const ConfigSectionSchema = z.object({
  key: z.string(),
  label: z.string(),
  summary: z.string().optional(),
  group_id: z.string(),
  fields: z.array(ConfigFieldSchema),
});
export type ConfigSection = z.infer<typeof ConfigSectionSchema>;

export const ConfigSchemaResponseSchema = z.object({
  sections: z.array(ConfigSectionSchema),
});
export type ConfigSchemaResponse = z.infer<typeof ConfigSchemaResponseSchema>;
