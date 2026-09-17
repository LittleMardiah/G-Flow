import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../services/api_client.dart';

final storageProvider = Provider((ref) => const FlutterSecureStorage());
final apiClientProvider = Provider((ref) => ApiClient());

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  return AuthNotifier(ref.watch(apiClientProvider), ref.watch(storageProvider));
});

class AuthState {
  final bool isLoading;
  final String? error;
  final Map<String, dynamic>? user;
  final bool isAuthenticated;

  AuthState({this.isLoading = false, this.error, this.user, this.isAuthenticated = false});

  AuthState copyWith({bool? isLoading, String? error, Map<String, dynamic>? user, bool? isAuthenticated}) {
    return AuthState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      user: user ?? this.user,
      isAuthenticated: isAuthenticated ?? this.isAuthenticated,
    );
  }
}

class AuthNotifier extends StateNotifier<AuthState> {
  final ApiClient apiClient;
  final FlutterSecureStorage storage;

  AuthNotifier(this.apiClient, this.storage) : super(AuthState());

  Future<void> login(String email, String password) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final res = await apiClient.post('/api/v1/auth/login', data: {'email': email, 'password': password});
      final data = res.data['data'];
      await storage.write(key: 'access_token', value: data['access_token']);
      await storage.write(key: 'refresh_token', value: data['refresh_token']);
      state = state.copyWith(isLoading: false, user: data, isAuthenticated: true);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: 'Login failed: ');
    }
  }

  Future<void> register(Map<String, dynamic> data) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      await apiClient.post('/api/v1/auth/register', data: data);
      state = state.copyWith(isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: 'Register failed: ');
    }
  }

  Future<void> logout() async {
    await storage.delete(key: 'access_token');
    state = AuthState();
  }
}
