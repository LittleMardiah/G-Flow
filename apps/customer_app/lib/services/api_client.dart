import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class ApiClient {
  static const String baseUrl = 'http://10.184.247.135:8080';
  final Dio dio;
  final FlutterSecureStorage storage;

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
          if (token != null) {
            options.headers['Authorization'] = 'Bearer ';
          }
          options.headers['X-Idempotency-Key'] = '';
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

  Future<Response> post(String path, {dynamic data}) => dio.post(path, data: data);
  Future<Response> get(String path) => dio.get(path);
  Future<Response> patch(String path, {dynamic data}) => dio.patch(path, data: data);
  Future<Response> delete(String path) => dio.delete(path);
}
