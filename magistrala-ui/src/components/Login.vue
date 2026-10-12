<template>
  <main class="auth-page">
    <p class="eyebrow">MAGISTRALA · AGRICULTURE</p>
    <h1>{{ signup ? tr('创建账号', 'Create account') : tr('欢迎回来', 'Welcome back') }}</h1>
    <p>
      {{
        tr(
          '连接设备、管理农田、按区域分析。',
          'Connect devices, manage fields and analyze each region.',
        )
      }}
    </p>
    <form @submit.prevent="submit">
      <label v-if="signup" class="field"
        >{{ tr('姓名', 'Name') }}<input v-model="name" required autocomplete="name" /></label
      ><label class="field"
        >{{ tr('邮箱或用户名', 'Email or username')
        }}<input v-model="username" required autocomplete="username" /></label
      ><label class="field"
        >{{ tr('密码', 'Password')
        }}<input
          v-model="password"
          required
          type="password"
          :autocomplete="signup ? 'new-password' : 'current-password'"
      /></label>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <p v-if="message" class="success">{{ message }}</p>
      <button :disabled="busy">
        {{
          busy ? tr('处理中…', 'Working…') : signup ? tr('注册', 'Sign up') : tr('登录', 'Sign in')
        }}
      </button>
    </form>
    <p>
      <router-link :to="signup ? '/login' : '/signup'">{{
        signup ? tr('返回登录', 'Back to sign in') : tr('注册账号', 'Create an account')
      }}</router-link>
    </p>
  </main>
</template>
<script setup>
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { gql, errorText } from '../api/platform'
import { signIn } from '../api/session'
import { tr } from '../i18n'
const route = useRoute(),
  router = useRouter(),
  signup = computed(() => route.path === '/signup')
const username = ref(''),
  password = ref(''),
  name = ref(''),
  busy = ref(false),
  error = ref(''),
  message = ref('')
async function submit() {
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    if (signup.value) {
      const data = await gql(
        'mutation($input:SignupInput!){signup(input:$input){entityId verificationRequired}}',
        { input: { name: name.value, email: username.value.trim(), password: password.value } },
        { auth: false },
      )
      message.value = data.signup.verificationRequired
        ? tr(
            '账号已创建，请先完成邮箱验证。',
            'Account created. Verify your email before signing in.',
          )
        : tr('账号已创建，可以登录。', 'Account created. You can sign in.')
      password.value = ''
    } else {
      await signIn(username.value.trim(), password.value)
      password.value = ''
      await router.push('/domains')
    }
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
