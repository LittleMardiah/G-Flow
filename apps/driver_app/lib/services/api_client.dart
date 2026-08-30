import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:uuid/uuid.dart';

import '../config/constants.dart';

class ApiClient {
  final Dio dio;
  final FlutterSecureStorage storage;
  final Uuid _uuid = const Uuid();

  ApiClient({Dio? dio, FlutterSecureStorage? storage})
      : dio = dio ?? Dio(BaseOptions(
          baseUrl: kApiBaseUrl,
          connectTimeout: const Duration(seconds: 60),
        )),
        storage = storage ?? const FlutterSecureStorage() {
    this.dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          final token = await this.storage.read(key: 'access_token');
          if (token != null && token.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          final idem = options.headers['X-Idempotency-Key'];
          if (idem == null || idem.toString().trim().isEmpty) {
            options.headers['X-Idempotency-Key'] = _uuid.v4();
          }
          return handler.next(options);
        },
        onError: (error, handler) async {
          if (error.response?.statusCode == 401) {
            await this.storage.delete(key: 'access_token');
          }
          return handler.next(error);
        },
      ),
    );
  }

  Future<Response> post(String path, {dynamic data, Map<String, dynamic>? headers}) =>
      dio.post(path, data: data, options: Options(headers: headers));
  Future<Response> get(String path, {Map<String, dynamic>? headers}) =>
      dio.get(path, options: Options(headers: headers));
  Future<Response> patch(String path, {dynamic data, Map<String, dynamic>? headers}) =>
      dio.patch(path, data: data, options: Options(headers: headers));
  Future<Response> delete(String path, {Map<String, dynamic>? headers}) =>
      dio.delete(path, options: Options(headers: headers));
}

String apiErrorMessage(Object error) {
  if (error is DioException) {
    final data = error.response?.data;
    if (data is Map && data['message'] != null) return '${data['message']}';
    if (error.message != null) return error.message!;
  }
  return error.toString();
}

/// Helper: parse `data` dari envelope `{ success, data }` backend.
Map<String, dynamic>? unwrapData(Object? raw) {
  if (raw is Map && raw['data'] is Map) {
    return (raw['data'] as Map).cast<String, dynamic>();
  }
  if (raw is Map<String, dynamic>) return raw;
  return null;
}

List<dynamic> unwrapList(Object? raw) {
  final data = raw is Map ? raw['data'] : raw;
  if (data is List) return data;
  if (data is Map && data['orders'] is List) return data['orders'] as List;
  if (data is Map && data['data'] is List) return data['data'] as List;
  return const <dynamic>[];
}