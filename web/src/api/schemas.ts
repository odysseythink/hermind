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
  'string', 'int', 'bool', 'secret', 'enum', 'float', 'multiselect',
]);
export type ConfigFieldKind = z.infer<typeof ConfigFieldKindSchema>;

export const ConfigPredicateSchema = z.object({
  field: z.string(),
  equals: z.unknown(),
});
export type ConfigPredicate = z.infer<typeof ConfigPredicateSchema>;

export const DatalistSourceSchema = z.object({
  section: z.string(),
  field: z.string(),
});
export type DatalistSource = z.infer<typeof DatalistSourceSchema>;

export const ConfigFieldSchema = z.object({
  name: z.string(),
  label: z.string(),
  help: z.string().optional(),
  kind: ConfigFieldKindSchema,
  required: z.boolean().optional(),
  default: z.unknown().optional(),
  enum: z.array(z.string()).optional(),
  visible_when: ConfigPredicateSchema.optional(),
  datalist_source: DatalistSourceSchema.optional(),
});
export type ConfigField = z.infer<typeof ConfigFieldSchema>;

export const ConfigSectionSchema = z.object({
  key: z.string(),
  label: z.string(),
  summary: z.string().optional(),
  group_id: z.string(),
  shape: z.enum(['map', 'scalar', 'keyed_map', 'list']).optional(), // default (absent) = map
  subkey: z.string().optional(),
  no_discriminator: z.boolean().optional(),
  fields: z.array(ConfigFieldSchema),
});
export type ConfigSection = z.infer<typeof ConfigSectionSchema>;

export const ConfigSchemaResponseSchema = z.object({
  sections: z.array(ConfigSectionSchema),
});
export type ConfigSchemaResponse = z.infer<typeof ConfigSchemaResponseSchema>;

export const ProviderModelsResponseSchema = z.object({
  models: z.array(z.string()),
});
export type ProviderModelsResponse = z.infer<typeof ProviderModelsResponseSchema>;

// ---- Chat mode (Phase 2/3) ----

export const MessageSubmitRequestSchema = z.object({
  text: z.string().min(1),
  model: z.string().optional(),
});
export type MessageSubmitRequest = z.infer<typeof MessageSubmitRequestSchema>;

export const MessageSubmitResponseSchema = z.object({
  session_id: z.string(),
  status: z.literal('accepted'),
});
export type MessageSubmitResponse = z.infer<typeof MessageSubmitResponseSchema>;

export const SessionSummarySchema = z.object({
  id: z.string(),
  title: z.string().optional(),
  source: z.string(),
  model: z.string().optional(),
  started_at: z.number().optional(),
  ended_at: z.number().optional(),
  message_count: z.number().optional(),
});
export type SessionSummary = z.infer<typeof SessionSummarySchema>;

export const SessionsListResponseSchema = z.object({
  sessions: z.array(SessionSummarySchema),
  total: z.number().optional(),
});
export type SessionsListResponse = z.infer<typeof SessionsListResponseSchema>;

export const ChatMessageSchema = z.object({
  id: z.string(),
  role: z.enum(['user', 'assistant', 'system', 'tool']),
  content: z.string(),
  timestamp: z.number().optional(),
  tool_calls: z.string().optional(),
});
export type ChatMessage = z.infer<typeof ChatMessageSchema>;

export const MessagesResponseSchema = z.object({
  messages: z.array(ChatMessageSchema),
  total: z.number().optional(),
});

export const StreamEventSchema = z.object({
  type: z.string(),
  session_id: z.string(),
  data: z.unknown().optional(),
});
export type StreamEvent = z.infer<typeof StreamEventSchema>;
