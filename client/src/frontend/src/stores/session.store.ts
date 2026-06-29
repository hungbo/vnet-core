import { defineStore } from 'pinia'
import { ref } from 'vue'

declare const window: any
const api = () => window.go?.main?.App

export const useSessionStore = defineStore('session', () => {
	const loggedIn = ref(false)
	const token = ref('')
	const userID = ref('')
	const machineCode = ref('')
	const role = ref('member')
	const displayName = ref('')
	const machineStatus = ref('available')
	const session = ref<any>(null)
	const memberInfo = ref<any>(null)
	const loginLoading = ref(false)

	async function login(username: string, password: string) {
		loginLoading.value = true
		try {
			const result = await api().Login(username, password)
			const data = JSON.parse(result)
			if (data?.access_token) {
				token.value = data.access_token
				userID.value = data.user?.id || ''
				role.value = data.user?.role || 'member'
				displayName.value = data.user?.full_name || data.user?.username || ''
				loggedIn.value = true
				localStorage.setItem('vnet_session', JSON.stringify({
					token: data.access_token,
					userId: data.user?.id,
					role: data.user?.role,
					displayName: data.user?.full_name || data.user?.username || ''
				}))
			}
			return data
		} finally {
			loginLoading.value = false
		}
	}

	async function logout() {
		localStorage.removeItem('vnet_session')
		loggedIn.value = false
		token.value = ''
		userID.value = ''
		machineCode.value = ''
		role.value = 'member'
		displayName.value = ''
		session.value = null
		memberInfo.value = null
		try { await api().Logout() } catch { /* ignore */ }
	}

	async function loadData() {
		machineCode.value = await api().GetMachineCode()
		const userInfoStr = await api().GetUserInfo()
		if (userInfoStr) {
			const info = JSON.parse(userInfoStr)
			role.value = info.role || 'member'
			displayName.value = info.full_name || info.username || ''
		}
		if (role.value !== 'admin') {
			const memberStr = await api().GetMemberInfo()
			if (memberStr) memberInfo.value = JSON.parse(memberStr)
			const sessionStr = await api().GetSession()
			if (sessionStr && sessionStr !== 'null') session.value = JSON.parse(sessionStr)
		}
	}

	async function updateSession(data: any) {
		session.value = data
	}

	async function updateBalance(data: any) {
		if (memberInfo.value) {
			if (data.balance !== undefined) memberInfo.value.balance = data.balance
			if (data.bonus_balance !== undefined) memberInfo.value.bonus_balance = data.bonus_balance
		}
	}

	async function restore() {
		const saved = localStorage.getItem('vnet_session')
		if (!saved) return false
		try {
			const s = JSON.parse(saved)
			if (!s.token) return false
			await api().RestoreSession(s.token, s.userId || '', '', s.displayName || '', s.role || 'member', '')
			token.value = s.token
			userID.value = s.userId || ''
			role.value = s.role || 'member'
			displayName.value = s.displayName || ''
			loggedIn.value = true
			return true
		} catch {
			localStorage.removeItem('vnet_session')
			return false
		}
	}

	return { loggedIn, token, userID, machineCode, role, displayName, machineStatus, session, memberInfo, loginLoading, login, logout, loadData, updateSession, updateBalance, restore }
})
