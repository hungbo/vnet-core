import type { PaginationData } from '@sa/hooks';

export function vnetTransform(response: any): PaginationData<any> {
  if (!response) {
    return { data: [], pageNum: 1, pageSize: 20, total: 0 };
  }
  return {
    data: response.items || [],
    pageNum: response.page || 1,
    pageSize: response.page_size || 20,
    total: response.total || 0
  };
}

export function vnetSimpleTransform(response: any): any[] {
  if (!response) return [];
  return response.items || (Array.isArray(response) ? response : []);
}
