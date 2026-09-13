import { request } from '../request';

/** get role list */
export function fetchGetRoleList(params?: Api.SystemManage.RoleSearchParams) {
  return request<Api.SystemManage.RoleList>({
    url: '/systemManage/getRoleList',
    method: 'get',
    params
  });
}

/**
 * get all roles
 *
 * these roles are all enabled
 */
export function fetchGetAllRoles() {
  return request<Api.SystemManage.AllRole[]>({
    url: '/systemManage/getAllRoles',
    method: 'get'
  });
}

/** get user list */
export function fetchGetUserList(params?: Api.SystemManage.UserSearchParams) {
  return request<Api.SystemManage.UserList>({
    url: '/systemManage/getUserList',
    method: 'get',
    params
  });
}

/** get menu list */
export function fetchGetMenuList(params?: Api.Common.CommonSearchParams) {
  return request<Api.SystemManage.MenuList>({
    url: '/systemManage/getMenuList/v2',
    method: 'get',
    params
  });
}

/** get all pages */
export function fetchGetAllPages() {
  return request<string[]>({
    url: '/systemManage/getAllPages',
    method: 'get'
  });
}

/** get menu tree */
export function fetchGetMenuTree() {
  return request<Api.SystemManage.MenuTree[]>({
    url: '/systemManage/getMenuTree',
    method: 'get'
  });
}

// --- Vai trò, người dùng, phân quyền ------------------------------------------
// Backend đã có sẵn các endpoint này từ trước; trang Vai trò và trang Người dùng
// của Soybean là bản mẫu — bấm Lưu chỉ hiện thông báo thành công rồi không gửi
// gì. Nối vào đây để hai trang đó nói thật.

export function fetchAddRole(body: Record<string, unknown>) {
  return request({ url: '/systemManage/addRole', method: 'post', data: body });
}

export function fetchUpdateRole(body: Record<string, unknown>) {
  return request({ url: '/systemManage/updateRole', method: 'post', data: body });
}

export function fetchDeleteRole(id: string) {
  return request({ url: '/systemManage/deleteRole', method: 'delete', data: { id } });
}

export function fetchBatchDeleteRole(ids: string[]) {
  return request({ url: '/systemManage/batchDeleteRole', method: 'delete', data: { ids } });
}

export function fetchAddUser(body: Record<string, unknown>) {
  return request({ url: '/systemManage/addUser', method: 'post', data: body });
}

export function fetchUpdateUser(body: Record<string, unknown>) {
  return request({ url: '/systemManage/updateUser', method: 'post', data: body });
}

export function fetchDeleteUser(id: string) {
  return request({ url: '/systemManage/deleteUser', method: 'delete', data: { id } });
}

export function fetchBatchDeleteUser(ids: string[]) {
  return request({ url: '/systemManage/batchDeleteUser', method: 'delete', data: { ids } });
}

/** Danh sách toàn bộ mã quyền hệ thống biết tới. */
export function fetchGetAllPermissions() {
  return request<Api.SystemManage.Permission[]>({
    url: '/systemManage/getAllPermissions',
    method: 'get'
  });
}

/** ID các quyền một vai trò đang có — dùng để tick sẵn. */
export function fetchGetRolePermissions(roleId: string) {
  return request<string[]>({
    url: '/systemManage/getRolePermissions',
    method: 'get',
    params: { roleId }
  });
}

/** Ghi đè toàn bộ danh sách quyền của vai trò. */
export function fetchUpdateRolePermissions(roleId: string, permissionIds: string[]) {
  return request({
    url: '/systemManage/updateRolePermissions',
    method: 'post',
    data: { roleId, permissionIds }
  });
}
