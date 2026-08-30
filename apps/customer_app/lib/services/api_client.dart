import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:uuid/uuid.dart';

class ApiClient {
  static const String baseUrl = 'http://10.184.247.135:8080';
  final Dio dio;
  final FlutterSecureStorage storage;
  final Uuid _uuid = const Uuid();

  ApiClient({Dio? dio, FlutterSecureStorage? storage})
      : dio = dio ?? Dio(BaseOptions(
          baseUrl: baseUrl,
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
          // Backend mewajibkan X-Idempotency-Key (API_CONTRACT). Jika caller
          // belum menyuplai key (misal lewat headers), buatkan key otomatis.
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