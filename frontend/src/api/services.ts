import api from './client';

export interface User {
  id: number;
  email: string;
  full_name: string;
  phone?: string;
}

export interface Trip {
  id: number;
  name?: string;
  title: string;
  start_location?: string;
  start_point: string;
  start_latitude?: number | null;
  start_longitude?: number | null;
  destination: string;
  destination_latitude?: number | null;
  destination_longitude?: number | null;
  stops?: TripStop[];
  start_date?: string;
  start_at: string;
  end_date?: string;
  end_at: string;
  estimated_budget?: number;
  budget_paise?: number;
  created_by?: number;
  status: string;
  creator_user_id?: number;
  invite_code?: string;
  user_role?: 'munimji' | 'co_reviewer' | 'participant';
  user_status?: 'invited' | 'active' | 'rejected' | 'left' | 'requested';
}

export interface TripStop {
  id?: number;
  position: number;
  name: string;
  latitude: number;
  longitude: number;
}

export interface PlaceSuggestion {
  id: string;
  name: string;
  latitude: number;
  longitude: number;
  nearby: boolean;
  distance_km?: number;
}

export interface PlaceSearchResult {
  suggestions: PlaceSuggestion[];
  nearby_unavailable: boolean;
}

export interface DashboardSummary {
  total_contributions_paise: number;
  group_funded_spend_paise: number;
  available_cash_paise: number;
  approved_claims_paise: number;
  pending_claims_paise: number;
}

export const authService = {
  login: (data: { email: string; password: string }) =>
    api.post<{ access_token: string; user: User }>('/auth/login', data),

  requestRegisterOTP: (email: string) =>
    api.post<{ message: string }>('/auth/register/otp', { email }),

  register: (data: { name: string; email: string; password: string; otp: string }) =>
    api.post<{ access_token: string; user: User }>('/auth/register', data),

  requestForgotPasswordOTP: (email: string) =>
    api.post<{ message: string }>('/auth/forgot-password/otp', { email }),

  resetPassword: (data: { email: string; otp: string; new_password: string }) =>
    api.post<{ access_token: string; user: User; message: string }>('/auth/forgot-password/reset', data),

  googleAuth: (data: { credential: string }) =>
    api.post<{ access_token: string; user: User }>('/auth/google', data),

  getMe: () =>
    api.get<User>('/auth/me'),

  updateProfile: (data: { name: string; phone?: string }) =>
    api.post<User>('/auth/profile/update', data),

  requestPasswordOTP: () =>
    api.post<{ message: string; target: string }>('/auth/password/otp'),

  changePassword: (data: { current_password?: string; otp?: string; new_password: string }) =>
    api.post<{ message: string }>('/auth/password/change', data),

  logout: () =>
    api.post<{ message: string }>('/auth/logout'),

  // Legacy
  requestOTP: (email: string) => api.post('/auth/otp/request', { email }),
  verifyOTP: (email: string, otp: string) => api.post<{ access_token: string; user: User }>('/auth/otp/verify', { email, otp }),
};

export interface TripMember {
  id: number;
  trip_id: number;
  user_id: number;
  user_name: string;
  email: string;
  role: 'munimji' | 'co_reviewer' | 'participant';
  status: 'active' | 'invited' | 'rejected' | 'left' | 'requested';
  created_at?: string;
  joined_at?: string;
}

export interface InviteInfo {
  trip_id: number;
  invite_code: string;
  title: string;
  start_point: string;
  destination: string;
  start_at: string;
  end_at: string;
  estimated_budget: number;
  munimji_name: string;
  munimji_email: string;
  member_count: number;
  user_status: '' | 'requested' | 'active' | 'rejected';
  is_munimji: boolean;
  joined_at: string;
}

export interface TripInvitation {
  id: number;
  trip_id: number;
  trip_title?: string;
  start_location?: string;
  destination?: string;
  inviter_user_id: number;
  inviter_name?: string;
  invitee_email: string;
  role: 'munimji' | 'co_reviewer' | 'participant';
  token: string;
  status: 'pending' | 'accepted' | 'rejected' | 'expired';
  expires_at?: string;
  responded_at?: string;
  created_at: string;
  updated_at?: string;
}

export interface EligibleUser {
  id: number;
  name: string;
  email: string;
}

export const tripService = {
  create: (data: {
    title: string;
    start_point: string;
    start_latitude: number;
    start_longitude: number;
    destination: string;
    destination_latitude: number;
    destination_longitude: number;
    start_at: string;
    end_at: string;
    estimated_budget: number;
    stops: Omit<TripStop, 'id' | 'position'>[];
  }) => api.post<Trip>('/trips/create', data),

  update: (data: {
    trip_id: number;
    title: string;
    start_point: string;
    start_latitude: number;
    start_longitude: number;
    destination: string;
    destination_latitude: number;
    destination_longitude: number;
    start_at: string;
    end_at: string;
    estimated_budget: number;
    stops: Omit<TripStop, 'id' | 'position'>[];
  }) => api.post<Trip>('/trips/update', data),

  list: () => api.post<Trip[]>('/trips/list', {}),

  getDetail: (trip_id: number) => api.post<{ trip: Trip; membership: any }>('/trips/detail', { trip_id }),

  join: (trip_id: number) => api.post('/trips/join', { trip_id }),

  getDashboard: (trip_id: number) => api.post<DashboardSummary>('/trips/dashboard', { trip_id }),

  listMembers: (trip_id: number) => api.post<TripMember[]>('/trips/members/list', { trip_id }),

  listEligibleUsers: (trip_id: number, query: string = '') =>
    api.post<EligibleUser[]>('/trips/members/eligible', { trip_id, query }),

  listInvitations: (trip_id: number) => api.post<TripInvitation[]>('/trips/invitations/list', { trip_id }),

  listMyPendingInvitations: () => api.post<TripInvitation[]>('/trips/invitations/my-pending', {}),

  inviteMember: (data: {
    trip_id: number;
    email: string;
    role: 'participant' | 'co_reviewer';
  }) => api.post('/trips/members/invite', data),

  generateReport: (trip_id: number) =>
    api.post('/trips/reports/generate', { trip_id }, { responseType: 'blob' }),

  updateStatus: (trip_id: number, status: 'planned' | 'active' | 'completed' | 'cancelled' | 'archived') =>
    api.post('/trips/status/update', { trip_id, status }),

  deleteTrip: (trip_id: number) =>
    api.post<{ message: string }>('/trips/delete', { trip_id }),

  getSettlement: (trip_id: number) =>
    api.post<TripSettlementResponse>('/trips/settlement/calculate', { trip_id }),

  respondInvitation: (trip_id: number, action: 'accept' | 'reject') =>
    api.post<{ message: string }>('/trips/invitation/respond', { trip_id, action }),

  removeMember: (data: { trip_id: number; user_id: number; reason: string }) =>
    api.post<{ message: string }>('/trips/members/remove', data),

  updateMemberRole: (data: { trip_id: number; user_id: number; role: 'co_reviewer' | 'participant' }) =>
    api.post<{ message: string; role: string }>('/trips/members/role/update', data),
  getInviteInfo: (code: string) =>
    api.get<InviteInfo>('/trips/invite-info', { params: { code } }),
  respondInviteLink: (data: { invite_code: string; action: 'request' | 'reject' }) =>
    api.post<{ message: string; action: string }>('/trips/invite-respond', data),
  listJoinRequests: (trip_id: number) =>
    api.post<TripMember[]>('/trips/members/requests', { trip_id }),
  decideJoinRequest: (data: { trip_id: number; user_id: number; action: 'approve' | 'reject' }) =>
    api.post<{ message: string; action: string }>('/trips/members/request-decision', data),
};

export const locationService = {
  search: (query: string, signal?: AbortSignal) =>
    api.get<PlaceSearchResult>('/locations/search', { params: { q: query }, signal }),
};


export interface SettlementTransfer {
  from_user_id: number;
  from_user_name: string;
  to_user_id: number;
  to_user_name: string;
  amount: number;
  amount_paise: number;
  transfer_type?: 'group_claim' | 'pool_refund' | 'peer_fair_share';
  description?: string;
}

export interface GroupClaimSettlement {
  claim_id: number;
  claimant_id: number;
  claimant_name: string;
  title: string;
  category: string;
  amount: number;
  amount_paise: number;
  owed_by: string;
  payable_from_pool: boolean;
}

export interface TreasurySettlementSummary {
  total_contributions_paise: number;
  group_spend_paise: number;
  available_cash_paise: number;
  approved_claims_paise: number;
  net_surplus_paise: number;
}

export interface ParticipantBalance {
  user_id: number;
  user_name: string;
  email: string;
  contributed: number;
  claimed: number;
  total_paid: number;
  equal_share: number;
  net_balance: number;
  net_balance_paise: number;
  pool_refund?: number;
  claim_payout?: number;
  peer_dues?: number;
}

export interface TripSettlementResponse {
  trip_id: number;
  total_trip_cost: number;
  total_trip_cost_paise: number;
  group_spend_paise: number;
  approved_claims_paise: number;
  equal_share_per_user: number;
  member_count: number;
  treasury_summary?: TreasurySettlementSummary;
  group_claims?: GroupClaimSettlement[];
  balances: ParticipantBalance[];
  transfers: SettlementTransfer[];
}

export interface ExpenseClaim {
  id: number;
  trip_id: number;
  claimant_user_id: number;
  amount_paise: number;
  currency: string;
  category: string;
  reason: string;
  expense_date: string;
  receipt_url?: string;
  status: 'pending' | 'needs_clarification' | 'approved' | 'rejected' | 'withdrawn';
  reimbursement_status: 'unpaid' | 'paid';
  reviewer_user_id?: number;
  decision_reason?: string;
  decided_at?: string;
  created_at: string;
}

export const claimService = {
  create: (data: {
    trip_id: number;
    amount?: number;
    amount_paise?: number;
    title?: string;
    category: string;
    reason: string;
    expense_date: string;
    receipt_url?: string;
    receipt_path?: string;
  }) => {
    const amountVal = data.amount ?? ((data.amount_paise ?? 0) / 100);
    const receipt = data.receipt_path || data.receipt_url;
    return api.post('/claims/create', {
      trip_id: data.trip_id,
      amount: amountVal,
      amount_paise: Math.round(amountVal * 100),
      title: data.title || data.category,
      category: data.category,
      reason: data.reason,
      expense_date: data.expense_date,
      receipt_path: receipt || null,
    });
  },

  list: (trip_id: number) => api.post<ExpenseClaim[]>('/claims/list', { trip_id }),

  approve: (trip_id: number, claim_id: number, reason?: string) =>
    api.post('/claims/approve', { trip_id, claim_id, reason }),

  reject: (trip_id: number, claim_id: number, reason?: string) =>
    api.post('/claims/reject', { trip_id, claim_id, reason }),
};

export const accountingService = {
  recordContribution: (data: {
    trip_id: number;
    user_id: number;
    amount?: number;
    amount_paise?: number;
    payment_method?: string;
    received_at: string;
    notes?: string;
  }) => {
    const amountVal = data.amount ?? ((data.amount_paise ?? 0) / 100);
    return api.post('/accounting/contributions/record', {
      ...data,
      amount: amountVal,
      amount_paise: Math.round(amountVal * 100),
    });
  },

  recordExpense: (data: {
    trip_id: number;
    title: string;
    category: string;
    amount?: number;
    amount_paise?: number;
    paid_from?: string;
    expense_date: string;
    notes?: string;
  }) => {
    const amountVal = data.amount ?? ((data.amount_paise ?? 0) / 100);
    return api.post('/accounting/expenses/record', {
      ...data,
      amount: amountVal,
      amount_paise: Math.round(amountVal * 100),
    });
  },

  listContributions: (trip_id: number) =>
    api.post<any[]>('/accounting/contributions/list', { trip_id }),

  listExpenses: (trip_id: number) =>
    api.post<any[]>('/accounting/expenses/list', { trip_id }),

  uploadReceipt: (file: File) => {
    const formData = new FormData();
    formData.append('receipt', file);
    return api.post<{ receipt_path: string; filename: string }>('/uploads/receipt', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
  },
};
