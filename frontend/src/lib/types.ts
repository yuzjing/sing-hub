// frontend/src/lib/types.ts
export type Strategy = 'append' | 'merge';

export interface Component {
  id: string;
  name: string;
  category: string;
  scope_tag: string;
  mount_path: string;
  strategy: Strategy;
  payload_tmpl: string;
  description: string;
  updated_at?: string;
}

export interface Module {
  id: string;
  template_id: string;
  name: string;
  category: string;
  params_json: string;
  description: string;
  updated_at?: string;
  template?: Component;
}

export interface ProfileModuleBinding {
  id?: string;
  profile_id?: string;
  module_id: string;
  order_index: number;
}

export interface ProfileCapabilities {
  enable_subscribe: boolean;
  enable_ping: boolean;
  enable_push: boolean;
}

export interface Profile {
  id: string;
  name: string;
  token: string;
  description: string;
  sing_box_version: string;
  capabilities: ProfileCapabilities;
  modules: ProfileModuleBinding[];
  updated_at?: string;
}

export interface Device {
  id: string;
  name: string;
  endpoints: string[];
  ssh_host: string;
  target_profile_id: string;
  auth_type: string;
  auth_secret: string;
  key_path: string;
  ssh_user: string;
  ssh_port: number;
  remote_path: string;
  tags: string[];
  enable_ping: boolean;
  enable_push: boolean;
  description: string;
  updated_at?: string;
}

export interface CommitRecord {
  hash: string;
  message: string;
  author: string;
  created_at: string;
}

export interface NodePingReport {
  device_id: string;
  online: boolean;
  latency_ms: number;
  error?: string;
}