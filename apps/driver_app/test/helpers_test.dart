import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/config/constants.dart';
import 'package:driver_app/services/api_client.dart';

void main() {
  group('formatRupiah', () {
    test('formats small amounts with thousands separators', () {
      expect(formatRupiah(0), 'Rp 0');
      expect(formatRupiah(500), 'Rp 500');
      expect(formatRupiah(8000), 'Rp 8.000');
      expect(formatRupiah(75000), 'Rp 75.000');
      expect(formatRupiah(1675000), 'Rp 1.675.000');
    });

    test('handles negative amounts', () {
      expect(formatRupiah(-5000), '-Rp 5.000');
    });
  });

  group('shortId', () {
    test('returns uppercased first 8 chars for long ids', () {
      expect(shortId('mock-ride-2351'), 'MOCK-RID');
      expect(shortId('abc12345'), 'ABC12345');
    });

    test('returns uppercased entire id for short ids', () {
      expect(shortId('abc'), 'ABC');
      expect(shortId(''), '');
      expect(shortId('12345678'), '12345678');
    });
  });

  group('apiErrorMessage', () {
    test('handles plain error objects', () {
      expect(apiErrorMessage(Exception('boom')), contains('boom'));
    });

    test('uses response message from DioException', () {
      final e = DioException(
        requestOptions: RequestOptions(path: '/x'),
        response: Response(
          requestOptions: RequestOptions(path: '/x'),
          statusCode: 400,
          data: {'message': 'bad request'},
        ),
      );
      expect(apiErrorMessage(e), 'bad request');
    });
  });

  group('unwrapData', () {
    test('extracts data from envelope', () {
      final map = unwrapData({'success': true, 'data': {'id': 1}});
      expect(map, {'id': 1});
    });

    test('returns raw map when no envelope', () {
      final map = unwrapData({'id': 2});
      expect(map, {'id': 2});
    });

    test('returns null for non-map raw', () {
      expect(unwrapData([1, 2]), isNull);
      expect(unwrapData('str'), isNull);
      expect(unwrapData(null), isNull);
    });
  });

  group('unwrapList', () {
    test('extracts list from data', () {
      expect(unwrapList({'data': [1, 2]}), [1, 2]);
    });

    test('extracts orders list', () {
      expect(unwrapList({'data': {'orders': ['a', 'b']}}), ['a', 'b']);
    });

    test('extracts nested data list', () {
      expect(unwrapList({'data': {'data': [1]}}), [1]);
    });

    test('returns the list as-is', () {
      expect(unwrapList([1, 2, 3]), [1, 2, 3]);
    });

    test('returns empty list for non-list', () {
      expect(unwrapList({'x': 1}), isEmpty);
      expect(unwrapList(null), isEmpty);
    });
  });
}
