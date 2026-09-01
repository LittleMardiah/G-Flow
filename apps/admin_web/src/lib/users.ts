"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type {
  AdminUser,
  UserListResponse,
  UserRole,
  UserStatus,
} from "@/lib/types";

export interface UserFilters {
  offset?: number;
  limit?: number;
  role?: UserRole | "ALL";
  status?: UserStatus | "ALL";
  search?: string;
}

export function useUsers(filters: UserFilters) {
  return useQuery({
    queryKey: ["users", filters],
    queryFn: async (): Promise<UserListResponse> => {
      const { data } = await api.get<UserListResponse>("/admin/users", {
        params: {
          ...filters,
          role: filters.role === "ALL" ? undefined : filters.role,
          status: filters.status === "ALL" ? undefined : filters.status,
          search: filters.search || undefined,
        },
      });
      return data;
    },
  });
}

export function useUserAction() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      userId,
      action,
    }: {
      userId: string;
      action: "freeze" | "suspend" | "ban" | "unfreeze";
    }) => {
      const { data } = await api.patch(`/admin/users/${userId}/${action}`);
      return data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUserDetail(userId: string | null) {
  return useQuery({
    queryKey: ["user", "detail", userId],
    enabled: !!userId,
    queryFn: async (): Promise<AdminUser> => {
      const { data } = await api.get<AdminUser>(`/admin/users/${userId}`);
      return data;
    },
  });
}
