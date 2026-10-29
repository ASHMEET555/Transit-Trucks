export interface WorkspaceOption {
  id: string
  name: string
}

export interface CompanyOption {
  id: string
  name: string
}

export const WORKSPACES: WorkspaceOption[] = [
  { id: 'default', name: 'Default Workspace' },
  { id: 'us-west', name: 'US West Operations' },
  { id: 'us-east', name: 'US East Operations' },
  { id: 'central', name: 'Central Logistics' },
]

export const COMPANIES: CompanyOption[] = [
  { id: 'transitops-primary', name: 'TransitOps Fleet' },
  { id: 'pacific-freight', name: 'Pacific Freight Lines' },
  { id: 'summit-logistics', name: 'Summit Express Logistics' },
]
