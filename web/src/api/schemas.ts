import { z } from 'zod';

// Config is shaped like the backend Config struct. We keep it largely
// opaque so unknown sub-sections round-trip on PUT unchanged.
export const ConfigSchema = z.object({}).catchall(z.unknown());
export type Config = z.infer<typeof ConfigSchema>;

export const ConfigResponseSchema = z.object({ config: ConfigSchema });

// Config section kinds produced by descriptor.FieldKind.String().
export const ConfigFieldKindSchema = z.enum([
  'string', 'int', 'bool', 'secret', 'enum', 'float', 'multiselect', 'text',
]);
export type ConfigFieldKind = z.infer<typeof ConfigFieldKindSchema>;

export const ConfigPredicateSchema = z.object({
  field: z.string(),
  equals: z.unknown().optional(),
  in: z.array(z.unknown()).optional(),
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
  shape: z.enum(['map', 'scalar', 'keyed_map', 'list']).optional(),
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

// ---- Chat (single-conversation) ----

export const StoredMessageSchema = z.object({
  id: z.number(),
  role: z.string(),
  content: z.string(),
  tool_call_id: z.string().optional(),
  tool_name: z.string().optional(),
  timestamp: z.number(),
  finish_reason: z.string().optional(),
  reasoning: z.string().optional(),
});
export type StoredMessage = z.infer<typeof StoredMessageSchema>;

export const ConversationHistoryResponseSchema = z.object({
  messages: z.array(StoredMessageSchema),
});
export type ConversationHistoryResponse = z.infer<typeof ConversationHistoryResponseSchema>;

export const ConversationPostResponseSchema = z.object({
  accepted: z.boolean(),
});
export type ConversationPostResponse = z.infer<typeof ConversationPostResponseSchema>;

export const MetaResponseSchema = z.object({
  version: z.string(),
  uptime_sec: z.number(),
  storage_driver: z.string(),
  instance_root: z.string(),
  current_model: z.string(),
});
export type MetaResponse = z.infer<typeof MetaResponseSchema>;

// Backend SSE events — the frontend matches on `type`.
export const StreamEventSchema = z.object({
  type: z.string(),
  data: z.unknown().optional(),
});
export type StreamEvent = z.infer<typeof StreamEventSchema>;
