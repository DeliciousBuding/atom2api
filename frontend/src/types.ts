export interface Token {
  id: number
  label: string
  access_token: string
  refresh_token?: string
  user_info: string
  status: string
  enabled: boolean
  quota_info: string
  last_used_at: string | null
  last_refreshed_at: string | null
  last_tested_at: string | null
  last_quota_at: string | null
  test_latency_ms: number
  error_message: string
  created_at: string
  updated_at: string
}

export interface APIKey {
  id: number
  name: string
  key: string
  enabled: boolean
  quota_limit: number
  quota_used: number
  expires_at: string | null
  created_at: string
}

export interface UsageLog {
  id: number
  token_id: number
  api_key_id: number
  model: string
  endpoint: string
  is_stream: boolean
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  status_code: number
  duration_ms: number
  error_message: string
  created_at: string
}

export interface Stats {
  tokens_total: number
  tokens_active: number
  tokens_error: number
  total_requests: number
  total_tokens: number
  api_keys_count: number
}
